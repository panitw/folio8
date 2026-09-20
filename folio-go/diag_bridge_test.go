package folio8

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// diagCodeBridgePins is the census's ANCHOR (D-000.87): one TEST-OWNED
// literal for every exported DiagCode* constant package folio8 declares.
//
// `bridged` is the constant itself and `literal` is this file's own
// independent spelling of the string it must equal. The two are compared
// below. What must never be written here is `bridged` compared against
// `diag.CodeX` — a constant compared against the thing it is defined from
// is trivially equal to itself, and THAT is the defect this census closes,
// not a weaker version of it.
//
// AD-14 makes changing a code's meaning a breaking change, and a bridge
// that alters one byte is exactly that change wearing a refactor: invisible
// in a diff that looks like a tidy-up, and invisible to the compiler, which
// is happy either way.
var diagCodeBridgePins = []struct {
	name    string // the constant's own identifier, cross-checked against the source
	bridged string // the constant
	literal string // this test's independent spelling of its value
}{
	{"DiagCodeTextClippedWidth", DiagCodeTextClippedWidth, "TEXT_CLIPPED_WIDTH"},
	{"DiagCodeEmptyAverage", DiagCodeEmptyAverage, "AGGREGATE_EMPTY_AVERAGE"},
	{"DiagCodeTextMissingGlyph", DiagCodeTextMissingGlyph, "TEXT_MISSING_GLYPH"},
	{"DiagCodeTextStyleFaceUndeclared", DiagCodeTextStyleFaceUndeclared, "TEXT_STYLE_FACE_UNDECLARED"},
	{"DiagCodeTextFaceAbsent", DiagCodeTextFaceAbsent, "TEXT_FACE_ABSENT"},
	{"DiagCodeTextFaceSubstituted", DiagCodeTextFaceSubstituted, "TEXT_FACE_SUBSTITUTED"},
	{"DiagCodeTableFooterSourceUnresolved", DiagCodeTableFooterSourceUnresolved, "TABLE_FOOTER_SOURCE_UNRESOLVED"},
	{"DiagCodeTableFooterSourceForbidden", DiagCodeTableFooterSourceForbidden, "TABLE_FOOTER_SOURCE_FORBIDDEN"},
	{"DiagCodeTemplateMalformed", DiagCodeTemplateMalformed, "TEMPLATE_MALFORMED"},
	{"DiagCodeBindingPathAbsent", DiagCodeBindingPathAbsent, "BINDING_PATH_ABSENT"},
	{"DiagCodeExpressionInvalid", DiagCodeExpressionInvalid, "EXPRESSION_INVALID"},
	{"DiagCodeContentUnlayoutable", DiagCodeContentUnlayoutable, "CONTENT_UNLAYOUTABLE"},
	{"DiagCodeInternalUnhandledCaveat", DiagCodeInternalUnhandledCaveat, "INTERNAL_UNHANDLED_CAVEAT"},
	{"DiagCodeDocumentDateInvalid", DiagCodeDocumentDateInvalid, "DOCUMENT_DATE_INVALID"},
	{"DiagCodeTableHeaderRepeatSuppressed", DiagCodeTableHeaderRepeatSuppressed, "TABLE_HEADER_REPEAT_SUPPRESSED"},
	{"DiagCodeTableFooterOrphanSuppressed", DiagCodeTableFooterOrphanSuppressed, "TABLE_FOOTER_ORPHAN_SUPPRESSED"},
	{"DiagCodeTableRowClippedHeight", DiagCodeTableRowClippedHeight, "TABLE_ROW_CLIPPED_HEIGHT"},
	{"DiagCodeTableMinHeightUnplaceable", DiagCodeTableMinHeightUnplaceable, "TABLE_MIN_HEIGHT_UNPLACEABLE"},
	{"DiagCodeTemplateFieldInvalid", DiagCodeTemplateFieldInvalid, "TEMPLATE_FIELD_INVALID"},
	{"DiagCodeBarcodeUnencodable", DiagCodeBarcodeUnencodable, "BARCODE_UNENCODABLE"},
	{"DiagCodeBarcodeModuleTooSmall", DiagCodeBarcodeModuleTooSmall, "BARCODE_MODULE_TOO_SMALL"},
	{"DiagCodeBarcodeDoesNotFit", DiagCodeBarcodeDoesNotFit, "BARCODE_DOES_NOT_FIT"},
	{"DiagCodeQRCodeTooLong", DiagCodeQRCodeTooLong, "QRCODE_TOO_LONG"},
	{"DiagCodeQRCodeModuleTooSmall", DiagCodeQRCodeModuleTooSmall, "QRCODE_MODULE_TOO_SMALL"},
	{"DiagCodeQRCodeDoesNotFit", DiagCodeQRCodeDoesNotFit, "QRCODE_DOES_NOT_FIT"},
	{"DiagCodeSectionBreakInvalid", DiagCodeSectionBreakInvalid, "SECTION_BREAK_INVALID"},
	{"DiagCodeSectionBreakStraddled", DiagCodeSectionBreakStraddled, "SECTION_BREAK_STRADDLED"},
	{"DiagCodeSectionBreakSplitsKeepTogether", DiagCodeSectionBreakSplitsKeepTogether, "SECTION_BREAK_SPLITS_KEEP_TOGETHER"},
	{"DiagCodePagesInvalid", DiagCodePagesInvalid, "PAGES_INVALID"},
}

// declaredDiagCodeConstants enumerates every exported DiagCode* CONSTANT
// package folio8 declares, by parsing its own non-test sources. Go has no
// runtime reflection over package-level constants, so the population is
// read from the source that defines it rather than from a list someone
// remembered to update — which is the whole difference between a census
// and the hand-written table this replaces.
//
// The scan is over the module root's whole non-test surface, not over
// diagnostic.go alone: a code minted in a new file is still a code, and a
// census keyed on a filename would miss it (the recurring "guard proves a
// filename property, not the real one" shape this repo has hit before).
func declaredDiagCodeConstants(t *testing.T) []string {
	t.Helper()
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}
	fset := token.NewFileSet()
	var names []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, perr := parser.ParseFile(fset, filepath.Join(root, name), nil, parser.SkipObjectResolution)
		if perr != nil {
			t.Fatalf("parse %s: %v", name, perr)
		}
		for _, d := range file.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, id := range vs.Names {
					if strings.HasPrefix(id.Name, "DiagCode") && ast.IsExported(id.Name) {
						names = append(names, id.Name)
					}
				}
			}
		}
	}
	return names
}

// TestEveryPublicDiagCodeBridgeIsPinnedToALiteral is D-000.87's CENSUS,
// owned by Story 4.6's finisher as the ruling names.
//
// The ruling: "Enumerate every public DiagCode* constant and require each
// to have a literal pin. The guard fires the moment a code is minted
// without one," plus "the enumeration must fail loudly if it finds ZERO
// constants."
//
// What this replaces is a hand-written five-entry table over a population
// of fifteen — the partial sweep, not the fix. Measured here by bisect
// before the census was written (each bridge rewritten in turn to a wrong
// literal and the whole suite re-run), SEVEN constants reddened NOTHING AT
// ALL:
//
//	DiagCodeTableFooterSourceForbidden   DiagCodeTemplateMalformed
//	DiagCodeBindingPathAbsent            DiagCodeExpressionInvalid
//	DiagCodeContentUnlayoutable          DiagCodeDocumentDateInvalid
//	and the render-time colour code D-7.8.2 later retired
//
// Seven, not the ruling's "ten": that figure was arithmetic over a
// name-grep, and "not literally pinned" and "reddens nothing under
// mutation" are different properties. Three of the ten — TextMissingGlyph,
// TableFooterSourceUnresolved and InternalUnhandledCaveat — are in fact
// caught indirectly, through TestAllProducedDiagnosticsCarryARegisteredCode
// and the cmd tests. Only the second property is what a census is for.
//
// And the substance is worse than the count. DiagCodeContentUnlayoutable is
// among the seven, and it is the code naming the REFUSED half of this
// story's own central claim: a row's height comes from data the author
// never saw and is forgiven, while a font size and an image box are things
// the author typed and are refused (AC8, D-4.6.2). That constant could have
// been repointed at any string and the entire suite would have compiled and
// passed. The census is not tidying — it closes a hole sitting directly
// under this story's own argument.
func TestEveryPublicDiagCodeBridgeIsPinnedToALiteral(t *testing.T) {
	declared := declaredDiagCodeConstants(t)

	// THE POPULATION GUARD, and it is the ruling's own second sentence.
	// A census whose population silently empties reports a clean sweep
	// over nothing — the inert-guard defect wearing a census's clothes,
	// and the exact shape this story spent two Blockers on elsewhere.
	if len(declared) == 0 {
		t.Fatal("census found ZERO exported DiagCode* constants in package folio8's non-test sources.\n\nThat is not a pass. Either the scan is broken (a moved file, a changed declaration form) or every public diagnostic code has been deleted. A census over an empty population reports exactly the all-clear a healthy one does, which is why D-000.87 requires this to be loud.")
	}

	pinned := make(map[string]string, len(diagCodeBridgePins))
	for _, p := range diagCodeBridgePins {
		if _, dup := pinned[p.name]; dup {
			t.Errorf("diagCodeBridgePins lists %s twice", p.name)
		}
		pinned[p.name] = p.literal
	}

	// (1) EVERY DECLARED CODE HAS A PIN. This is the limb that fires the
	// moment a code is minted without one — a new `const DiagCodeX =
	// string(diag.CodeX)` reddens here on the commit that adds it, with
	// no one needing to remember this file exists.
	seen := map[string]bool{}
	for _, name := range declared {
		seen[name] = true
		if _, ok := pinned[name]; !ok {
			t.Errorf("%s is an exported DiagCode* constant with NO literal pin in diagCodeBridgePins.\n\nAD-14 makes a code's string its public meaning, so a bridge that changes one byte is a breaking change that no compiler and no existing test can see. Add a row to diagCodeBridgePins spelling the expected string as a LITERAL — never as a comparison against diag.Code…, which is the constant compared against itself.", name)
		}
	}

	// (2) AND EVERY PIN NAMES A REAL CODE. Without this, a pin left
	// behind by a deleted constant makes the table look more complete
	// than the population it claims to cover.
	for _, p := range diagCodeBridgePins {
		if !seen[p.name] {
			t.Errorf("diagCodeBridgePins pins %q, which is not an exported DiagCode* constant in package folio8's non-test sources — a stale pin inflates the census's apparent coverage", p.name)
		}
	}

	// (3) AND THE BRIDGE MATCHES ITS PIN. The pins are literals this file
	// owns, independent of both diagnostic.go and internal/diag/diag.go:
	// if either side drifts, this reddens even though the two source
	// files would still compile happily against each other.
	for _, p := range diagCodeBridgePins {
		if p.bridged != p.literal {
			t.Errorf("%s = %q, want %q", p.name, p.bridged, p.literal)
		}
	}

	// The count is reported rather than asserted: an exact-population
	// assertion would be a conservation claim, and minting a code is a
	// legitimate, expected act. Limb (1) is what makes the new code
	// arrive with a pin; nothing here should make it arrive with a
	// second edit to a magic number.
	t.Logf("census: %d exported DiagCode* constants, %d pinned", len(declared), len(diagCodeBridgePins))
}
