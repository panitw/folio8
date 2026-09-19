package template

import (
	"regexp"
	"strings"
)

// The closed sets folio-format.md states, enforced at load (AC5). Kept
// visibly separate from passthrough (AC10, AC11 clause 3): an unknown
// KEY passes through opaquely; a recognised key carrying an unlisted
// VALUE from one of these sets is still a load error. Conflating the
// two paths is how passthrough silently becomes "validation is
// optional" — every check against one of these sets lives here, and
// nowhere near the passthrough code in parse.go.

// closedLocales moved to locale.go (Story 3.4, AC4): the closed set of
// valid `locale` tags is now built from named constants exported for
// internal/expr's locale table to key off, rather than a second copy
// of the tag literals.

var closedElementTypes = map[string]bool{
	"text": true, "image": true, "table": true, "line": true, "rect": true, "barcode": true, "qrcode": true,
}

// QRErrorCorrectionTokens is the closed set a qrcode's `errorCorrection`
// admits, in the order a refusal names them. A slice, looked up by
// IsQRErrorCorrection, so the refusal message is derived from the set that
// is enforced and no map is ranged.
var QRErrorCorrectionTokens = []string{"L", "M", "Q", "H"}

// QRErrorCorrectionDefault is the level a qrcode without the key encodes at.
const QRErrorCorrectionDefault = "M"

// IsQRErrorCorrection reports whether s is a member of the error-correction
// set. Exported for the property command, so the command door and the file
// door admit exactly the same values.
func IsQRErrorCorrection(s string) bool {
	for _, token := range QRErrorCorrectionTokens {
		if s == token {
			return true
		}
	}
	return false
}

var closedPageOrientations = map[string]bool{
	"portrait": true, "landscape": true,
}

var closedPageSizeNames = map[string]bool{
	"A4": true, "Letter": true,
}

// THE ALIGNMENT VOCABULARY IS THREE SETS, PARTITIONED BY CONSUMER
// (Story 7.3 D-7.3.1, re-partitioned at Story 7.8).
//
// Before Story 7.3 one shared map served every align field, so
// extending it with `justify` would have silently legalised justified
// TABLE CELLS — a scope decision nobody made, arriving as a side effect
// of a map edit. Story 7.3 split it in two.
//
// IT SPLIT IT ALONG THE WRONG SEAM, and that is the reusable lesson.
// The split was BY JSON KEY LOCATION — `style`/`headerStyle` on one
// side, `columns[]` on the other — but a table's `style.align` and its
// `columns[].align` are read into the SAME variable
// (table_render.go's r.alignFallback) and consumed at the SAME four
// switches. They are one consumer wearing two key paths, so the
// guardrail meant to make justified table cells impossible let the
// value straight in through the other door: a table declaring
// `style.align: "justify"` loaded, paid a MAJOR version bump, and drew
// every cell at the start edge with no diagnostic (DW-29).
//
//	WHEN SPLITTING A CLOSED SET, PARTITION IT BY THE CODE THAT
//	CONSUMES THE VALUE, NOT BY WHERE THE VALUE IS WRITTEN IN THE
//	DOCUMENT.
//
// So the sets are keyed on the CONSUMER, which here means the ELEMENT
// TYPE that owns the style block:
//
//   - StyleAlignTokens — a NON-TABLE element's own `style.align`. The
//     paragraph justifier consumes it, and `justify` (FR47) means
//     something there.
//   - TableStyleAlignTokens — a TABLE's `style.align` and
//     `headerStyle.align`. The table cell renderer consumes it, through
//     exactly the same alignFallback as the column set, so it admits
//     exactly what that consumer can draw.
//   - ColumnAlignTokens — `columns[].align`. Same consumer, same three
//     values; kept as its own declaration because it is a separately
//     documented closed set in the format.
//
// The three are SEPARATE DECLARATIONS and none is derived from another
// by subtraction at a call site.
//
// The tokens are named constants, and the ordered slices are built from
// THEM rather than from a second set of string literals — locale.go's
// LocaleTags/closedLocales precedent (D-1.3.5). The slices exist because
// ranging a map anywhere under internal/ is a BUILD FAILURE, and because
// the rejection MESSAGE is derived from the slice rather than restated
// as a literal: after Story 7.3 no message may claim a set of legal
// values that differs from the set actually enforced. That invariant is
// what forced a third SET rather than a type-guard bolted above the
// existing two — under a bare guard a table's `align: "middle"` would
// still be rejected by the style map and would report `left, center,
// right, justify`, naming as legal a value that element can never carry.
const (
	AlignLeft    = "left"
	AlignCenter  = "center"
	AlignRight   = "right"
	AlignJustify = "justify"
)

// StyleAlignTokens is the closed set a NON-TABLE element's own
// `style.align` admits, in the exact order the load error lists them.
// `justify` (FR47) is in THIS set and no other — extending a closed set
// is a MAJOR version change under D-1.4.12, which is why the format
// moved to 2.0 in the same story, and it is why declaring it is the one
// thing in the format that raises a document to 2.0.
//
// It is NOT what `headerStyle.align` admits, and not what a table's own
// `style.align` admits: those two attachment points belong to a table,
// whose consumer is TableStyleAlignTokens below (Story 7.8). The
// sentence this doc comment carried until then — "the closed set
// `style.align` and `headerStyle.align` admit" — described the key
// path, which is exactly the partition that shipped the defect.
var StyleAlignTokens = []string{AlignLeft, AlignCenter, AlignRight, AlignJustify}

// TableStyleAlignTokens is the closed set a TABLE's `style.align` and
// `headerStyle.align` admit (Story 7.8). It is the column triple, and
// it is the column triple for a reason that is not coincidence: those
// two attachment points and `columns[].align` are read into one
// alignFallback and consumed at one set of switches, so they must admit
// exactly what that consumer can draw. Justified table cells remain a
// separate scope decision, and this is the set that actually makes them
// impossible.
//
// A separate declaration from ColumnAlignTokens rather than an alias of
// it: they are separately documented closed sets in folio-format.md,
// and either could move without the other. TestAlignSetsAreThreeSets…
// pins them against their own maps and against each other.
var TableStyleAlignTokens = []string{AlignLeft, AlignCenter, AlignRight}

// ColumnAlignTokens is the closed set `columns[].align` admits. It keeps
// exactly the three values it has always had: justified table cells are
// a separate scope decision.
var ColumnAlignTokens = []string{AlignLeft, AlignCenter, AlignRight}

var closedStyleAligns = map[string]bool{
	AlignLeft: true, AlignCenter: true, AlignRight: true, AlignJustify: true,
}

var closedTableStyleAligns = map[string]bool{
	AlignLeft: true, AlignCenter: true, AlignRight: true,
}

var closedColumnAligns = map[string]bool{
	AlignLeft: true, AlignCenter: true, AlignRight: true,
}

// ColumnHeaderAlignTokens is the closed set `columns[].headerAlign` admits.
// The same three values as the column set, and the same consumer (a table
// cell), but a SEPARATE declaration, as the three align sets above are: it is
// separately documented in the format, and extending one must never legalise
// another by accident.
var ColumnHeaderAlignTokens = []string{AlignLeft, AlignCenter, AlignRight}

var closedColumnHeaderAligns = map[string]bool{
	AlignLeft: true, AlignCenter: true, AlignRight: true,
}

// IsColumnHeaderAlign reports whether s is a member of the column header
// alignment set. Exported for the updateTableColumn command, so the command
// door and the file door admit exactly the same values.
func IsColumnHeaderAlign(s string) bool { return closedColumnHeaderAligns[s] }

// IsStyleAlign reports whether s is a member of a NON-TABLE element's
// style alignment set. Exported for the property-command path
// (component_commands.go), which sets style.align from a command and
// must validate it against the same single source the loader does
// rather than against a second literal.
func IsStyleAlign(s string) bool { return closedStyleAligns[s] }

// IsTableStyleAlign reports whether s is a member of a TABLE's style
// alignment set — its own `style.align` and its `headerStyle.align`.
// Exported for the same reason and under the same obligation as
// IsStyleAlign: the property-command path selects between the two on
// the element's type, exactly as decodeStyle does, so the command door
// and the file door cannot admit different values (Story 7.8). Without
// it the designer's engine could still stamp a table document 2.0 with
// a value the loader would then refuse to reopen.
func IsTableStyleAlign(s string) bool { return closedTableStyleAligns[s] }

// closedSetMessage renders an ordered token slice as the tail of a
// closed-set load error. Derived from the slice, never hand-written, so
// a set that gains or loses a member cannot ship a message that lies
// about what is legal (internal/expr/locale.go:lookupLocale's precedent,
// which formats template.LocaleTags into its own message the same way).
func closedSetMessage(tokens []string) string {
	return "not one of the closed set " + strings.Join(tokens, ", ")
}

// StyleValignTokens is the closed set `style.valign` and a table's
// `headerStyle.valign` admit, in the order a refusal names them. One
// declaration, exactly as the three align sets are: the loader's own
// message below is DERIVED from it (closedSetMessage), and the command
// door asks IsStyleValign rather than restating the triple.
var StyleValignTokens = []string{"top", "middle", "bottom"}

// closedValigns is StyleValignTokens as a lookup, built from the slice
// so the set and the sentence that reports it cannot drift apart.
var closedValigns = func() map[string]bool {
	set := make(map[string]bool, len(StyleValignTokens))
	for _, token := range StyleValignTokens {
		set[token] = true
	}
	return set
}()

// IsStyleValign reports whether s is a member of the vertical-alignment
// set. Exported for the command path (component_commands.go's Story
// 12.3 header-style arm), which writes headerStyle.valign and owes the
// author a located refusal drawn from the SAME source the loader reads
// — never a second literal triple that could legalise a value the file
// door still refuses.
func IsStyleValign(s string) bool { return closedValigns[s] }

// BorderEdgeTokens is the closed set `style.border.edges` and a table's
// `headerStyle.border.edges` admit, in the order a refusal names them —
// and in the order the projection joins them. One declaration, exactly
// as StyleValignTokens is: the loader's own refusal reads the lookup
// derived from it below, and the command door asks IsBorderEdge rather
// than restating the four names.
var BorderEdgeTokens = []string{"top", "right", "bottom", "left"}

// closedBorderEdges is BorderEdgeTokens as a lookup, built from the
// slice so the set and the sentence that reports it cannot drift apart.
var closedBorderEdges = func() map[string]bool {
	set := make(map[string]bool, len(BorderEdgeTokens))
	for _, token := range BorderEdgeTokens {
		set[token] = true
	}
	return set
}()

// IsBorderEdge reports whether s is a member of the border-edge set.
// Exported for the command path (component_commands.go's Story 14.8
// `headerStyle.border.edges` arm), which owes the author a LOCATED
// refusal drawn from the SAME source the loader reads. Without it an
// unknown edge name is admitted at the command door, mutates the
// document, and surfaces later as an unlocated ParseTemplate failure off
// the wasm round-trip — naming no element and no field, exactly the
// failure the border.width and border.color arms restate the loader's
// rules to avoid.
func IsBorderEdge(s string) bool { return closedBorderEdges[s] }

// RuleBoundaryTokens is the closed set `table.rules.between` admits, in
// the order a refusal names them — and the order the projection joins
// them. One declaration, exactly as BorderEdgeTokens is.
//
// ⚠ THIS SET IS EFFECTIVELY PERMANENT FROM THE COMMIT THAT SHIPPED IT.
// Extending a closed set is a MAJOR version change under D-1.4.12, so a
// third boundary name cannot be added without a major bump. It was
// closed deliberately (SPEC-table-rules, "Ask First"): a table has
// exactly two families of interior boundary, one per axis, and a value
// outside the pair would name something that does not exist.
//
// It is NOT BorderEdgeTokens and must never be derived from it. An edge
// is one of a CELL's four sides; a boundary is one of the TABLE's
// interior seams. Conflating the two vocabularies is the defect this
// block replaces.
var RuleBoundaryTokens = []string{"columns", "rows"}

// closedRuleBoundaries is RuleBoundaryTokens as a lookup, built from the
// slice so the set and the sentence that reports it cannot drift apart.
var closedRuleBoundaries = func() map[string]bool {
	set := make(map[string]bool, len(RuleBoundaryTokens))
	for _, token := range RuleBoundaryTokens {
		set[token] = true
	}
	return set
}()

// IsRuleBoundary reports whether s is a member of the rule-boundary set.
// Exported for the command path (component_commands.go's `rules` arm),
// which owes the author a LOCATED refusal drawn from the SAME source the
// loader reads — never a second literal pair that could admit at the
// command door a value the file door still refuses.
func IsRuleBoundary(s string) bool { return closedRuleBoundaries[s] }

var closedFooterKinds = map[string]bool{
	"sum": true, "count": true, "avg": true,
}

// UTCOffsetSyntax is the ONE spelling of what `utcOffset` must look
// like. The loader's own refusal and the command door's both render it
// (parse.go, component_commands.go's setDocumentUTCOffset), so the
// sentence the author reads cannot drift from the pattern below by a
// re-typing. The phrase comes from folio-format.md's own
// `utcOffset` row in the top-level field table: "utcOffset | Fixed
// offset, ±HH:MM." (Cited by NAME rather than by line: this story
// edits that file, and a line number a story's own edit can move is
// the failure mode D-12.C.4's generalisation names.)
const UTCOffsetSyntax = "±HH:MM"

// utcOffsetPattern is ±HH:MM (AC5), AND IT ENFORCES THE RANGE (D-12.C).
//
// It used to be `^[+-][0-9]{2}:[0-9]{2}$`, which admitted `+99:99` — a
// value no clock has and internal/expr's parseUTCOffsetMinutes then
// refused, so the document LOADED and every formatDate in it failed at
// render with `expr: invalid UTC offset "+99:99"`. D-12.C ruled that
// repairing this is implementing folio-format.md's `utcOffset`
// field-table row rather than
// narrowing it: `+99:99` is not a fixed offset, and the format's
// compatibility rules are recorded as explicitly undefined pending a
// policy (SPEC.md's open-questions list; epics.md's NFR6).
//
// NOTHING REAL IS EXCLUDED (D-12.C.3): measured over every `.folio`
// file `git ls-files --others --cached` reports — 31 of them — 24
// declare `+00:00`, 7 declare `+07:00`, and ZERO are excluded by the
// repair. D-12.C's own table says 28/21/7; that was a smaller
// population (tracked files only), not a different answer.
//
// HH is an hour (00–23) and MM a minute (00–59), so `+24:00` is refused
// too.
//
// `Z` IS NOT ADMITTED HERE, ON ONE GROUND: it is not `±HH:MM`.
// folio-format.md's `utcOffset` field-table row states the field's
// syntax and `Z` does not match
// it, so excluding it IMPLEMENTS the format exactly as excluding
// `+99:99` does. `Z` is RFC 3339's UTC spelling and belongs to report
// DATA, where internal/expr's parseUTCOffsetMinutes serves it (D-12.C).
//
// `-00:00` IS ADMITTED, and the contrast is worth naming because it
// looks like an inconsistency and is not: `-00:00` IS `±HH:MM` and `Z`
// is not. Syntax is the whole test. (An earlier draft of this comment
// argued `Z` out on the ground that a second byte spelling of one
// semantic offset is a hazard in a byte-identity product. D-12.C.4
// WITHDRAWS that argument: it proves too much — it would condemn
// `-00:00`, and so the very pattern D-12.C ruled correct — and it was
// never true here anyway, because values travel VERBATIM. The
// serializer normalises no offset, so `-00:00` round-trips as
// `-00:00`; a canonicalization hazard needs a canonicalizer, and this
// product has none.)
var utcOffsetPattern = regexp.MustCompile(`^[+-](?:[01][0-9]|2[0-3]):[0-5][0-9]$`)

// IsUTCOffset reports whether s is a legal `utcOffset` — the SINGLE
// predicate both doors ask. The loader calls it (parse.go) and so does
// the command path (component_commands.go's setDocumentUTCOffset),
// under the same obligation and for the same reason as IsStyleAlign and
// IsLocale: a command whose accept-set differs from the loader's, in
// either direction, is a document the engine can stamp and then refuse
// to reopen — or a value the file admits and the panel cannot restate.
//
// D-12.C: command and loader agree BY CONSTRUCTION, not by care.
func IsUTCOffset(s string) bool { return utcOffsetPattern.MatchString(s) }
