package diag

import "testing"

// codePins is AC5's test-owned pin: {constant identifier -> exact
// string literal} for EVERY currently-shipped code (R3). It is
// deliberately hand-typed here and NEVER derived from, generated from,
// or imported out of the registry source in diag.go — if both sides
// moved together the instrument would be tautological (D-3.4.6's
// date-token failure, D-000.68). This is the anchor: a literal the
// test owns.
//
// Finding 2 (QA review, Major): the original shape kept the
// identifier only as a map key of type string, used solely inside
// t.Errorf format arguments — every assertion actually ran over the
// pinned VALUES as a set, and a set comparison is permutation-invariant.
// Swapping two shipped codes' strings (e.g. CodeTemplateMalformed and
// CodeBindingPathAbsent trading meanings — exactly the breaking change
// AD-14 names and AC5 exists to catch) left the value set unchanged, so
// nothing reddened. Each entry here instead carries the NAMED CONSTANT
// itself, read live, so the identifier->string binding is what gets
// compared — a swap changes which literal a given constant equals,
// which this catches regardless of what the overall value set looks
// like.
var codePins = []struct {
	name     string // for error messages only
	constant Code   // the actual named constant, read live from diag.go
	literal  Code   // the pinned expected string
}{
	{"CodeTextClippedWidth", CodeTextClippedWidth, "TEXT_CLIPPED_WIDTH"},
	{"CodeEmptyAverage", CodeEmptyAverage, "AGGREGATE_EMPTY_AVERAGE"},
	{"CodeTableFooterSourceUnresolved", CodeTableFooterSourceUnresolved, "TABLE_FOOTER_SOURCE_UNRESOLVED"},
	{"CodeTableFooterSourceForbidden", CodeTableFooterSourceForbidden, "TABLE_FOOTER_SOURCE_FORBIDDEN"},
	{"CodeTemplateMalformed", CodeTemplateMalformed, "TEMPLATE_MALFORMED"},
	{"CodeBindingPathAbsent", CodeBindingPathAbsent, "BINDING_PATH_ABSENT"},
	{"CodeExpressionInvalid", CodeExpressionInvalid, "EXPRESSION_INVALID"},
	{"CodeContentUnlayoutable", CodeContentUnlayoutable, "CONTENT_UNLAYOUTABLE"},
	{"CodeTextMissingGlyph", CodeTextMissingGlyph, "TEXT_MISSING_GLYPH"},
	{"CodeTextStyleFaceUndeclared", CodeTextStyleFaceUndeclared, "TEXT_STYLE_FACE_UNDECLARED"},
	{"CodeTextFaceAbsent", CodeTextFaceAbsent, "TEXT_FACE_ABSENT"},
	{"CodeTextFaceSubstituted", CodeTextFaceSubstituted, "TEXT_FACE_SUBSTITUTED"},
	{"CodeInternalUnhandledCaveat", CodeInternalUnhandledCaveat, "INTERNAL_UNHANDLED_CAVEAT"},
	{"CodeDocumentDateInvalid", CodeDocumentDateInvalid, "DOCUMENT_DATE_INVALID"},
	{"CodeTableHeaderRepeatSuppressed", CodeTableHeaderRepeatSuppressed, "TABLE_HEADER_REPEAT_SUPPRESSED"},
	{"CodeTableFooterOrphanSuppressed", CodeTableFooterOrphanSuppressed, "TABLE_FOOTER_ORPHAN_SUPPRESSED"},
	{"CodeTableRowClippedHeight", CodeTableRowClippedHeight, "TABLE_ROW_CLIPPED_HEIGHT"},
	{"CodeTableMinHeightUnplaceable", CodeTableMinHeightUnplaceable, "TABLE_MIN_HEIGHT_UNPLACEABLE"},
	{"CodeTemplateFieldInvalid", CodeTemplateFieldInvalid, "TEMPLATE_FIELD_INVALID"},
	{"CodeBarcodeUnencodable", CodeBarcodeUnencodable, "BARCODE_UNENCODABLE"},
	{"CodeBarcodeModuleTooSmall", CodeBarcodeModuleTooSmall, "BARCODE_MODULE_TOO_SMALL"},
	{"CodeBarcodeDoesNotFit", CodeBarcodeDoesNotFit, "BARCODE_DOES_NOT_FIT"},
	{"CodeQRCodeTooLong", CodeQRCodeTooLong, "QRCODE_TOO_LONG"},
	{"CodeQRCodeModuleTooSmall", CodeQRCodeModuleTooSmall, "QRCODE_MODULE_TOO_SMALL"},
	{"CodeQRCodeDoesNotFit", CodeQRCodeDoesNotFit, "QRCODE_DOES_NOT_FIT"},
	{"CodeSectionBreakInvalid", CodeSectionBreakInvalid, "SECTION_BREAK_INVALID"},
	{"CodeSectionBreakStraddled", CodeSectionBreakStraddled, "SECTION_BREAK_STRADDLED"},
	{"CodeSectionBreakSplitsKeepTogether", CodeSectionBreakSplitsKeepTogether, "SECTION_BREAK_SPLITS_KEEP_TOGETHER"},
	{"CodePagesInvalid", CodePagesInvalid, "PAGES_INVALID"},
}

// TestRegistryIsAdditiveOnly is AC5: additive-only is enforced by
// pinning each code's STRING to a test-owned literal, never by pinning
// the registry's SIZE (R3, D-000.68). Three things, all against the
// registry AS CONSTRUCTED (All(), which reads the package's own
// registry map, never allCodes or the const block directly):
//
//   - the named constant still equals the literal this test pins it to
//     (catches a changed OR REPURPOSED code — including two codes
//     swapping strings, Finding 2's red-proof — the breaking change
//     AD-14 forbids and the reason this AC exists);
//   - every pinned literal is a registry member (catches a pin whose
//     code was removed or never registered);
//   - every registry member appears in the pin table (catches a code
//     shipped without a pin).
//
// Growing the registry with a new, PINNED code must not redden this
// test — see the mutation record in the story's Delivery Log for the
// mutations this test was built against (change a string,
// add-without-pin, add-with-pin, swap two strings) and their observed
// results.
//
// ONE DATED EXCEPTION TO "ADDITIVE ONLY" (2026-09-17). D-7.8.2 retired
// the render-time colour code and the line-spacing load code before the
// folio-go/v1.0.0 tag, the one moment AD-14 lets a code be removed
// without a breaking change: no consumer branched on either. Their pins
// were removed with their constants. After the tag this test's rule is
// absolute again.
func TestRegistryIsAdditiveOnly(t *testing.T) {
	pinnedLiterals := make(map[Code]bool, len(codePins))
	for _, p := range codePins {
		if p.constant != p.literal {
			t.Errorf("%s = %q, want pinned literal %q — a shipped code's string changed (AD-14: a breaking change)", p.name, p.constant, p.literal)
		}
		if !Registered(p.literal) {
			t.Errorf("pinned code %s (%q) is not a member of the registry as constructed", p.name, p.literal)
		}
		pinnedLiterals[p.literal] = true
	}

	for _, c := range All() {
		if !pinnedLiterals[c] {
			t.Errorf("registry member %q has no entry in this test's pin table", c)
		}
	}

	if got, want := len(All()), len(codePins); got != want {
		t.Errorf("registry has %d member(s), pin table has %d — every registry member must appear in the table and vice versa", got, want)
	}
}

// _codeIsADefinedType is R4/AC1's compiler anchor, pinned at compile
// time rather than as a runtime Test (Finding 18, QA review, Nit): the
// previous TestCodeIsADefinedType's body — assign a literal to a Code
// variable, then compare it to itself — could not fail while the
// package compiled, which its own comment already conceded. Code is
// `type Code string`, so this line fails to COMPILE, not to run, the
// moment Code stops being string-based; that is the actual anchor, and
// a compile-time assertion states it without a passing-but-toothless
// test inflating the suite's count.
var _ Code = Code("compile-time anchor: Code must stay string-based (R4)")
