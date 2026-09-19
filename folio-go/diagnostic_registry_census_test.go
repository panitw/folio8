package folio8

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/diag"
	"github.com/panitw/folio8/folio-go/internal/expr"
)

// TestDiagnosticRegistryErrorCensus derives its worklist from the constructed
// registry. A new registered Error therefore cannot pass merely because a
// hand-maintained subset forgot it: it needs a disposition and a real
// production trigger before the suite is green.
func TestDiagnosticRegistryErrorCensus(t *testing.T) {
	for _, code := range diag.All() {
		if disposition, ok := diag.Classified(code); !ok || (disposition != diag.DispositionError && disposition != diag.DispositionWarning) {
			t.Fatalf("registered code %q has no valid registry disposition", code)
		}
	}

	triggers := map[diag.Code]func(*testing.T) error{
		diag.CodeTemplateMalformed: func(t *testing.T) error {
			// Story 7.8: this trigger used to be malformedTemplateJSON —
			// a well-formed document missing a required field — which is
			// TEMPLATE_FIELD_INVALID now. The production condition this
			// code still names is a document that is not a `.folio`
			// document at all, which is the one whose message may quote
			// the input back.
			_, err := ParseTemplate([]byte(unparseableTemplateJSON))
			return err
		},
		diag.CodeTemplateFieldInvalid: func(t *testing.T) error {
			// Story 7.8, D-7.8.1. The GENERAL load-stage condition, and
			// the census's own reason for existing applies to it as much
			// as to any specific code: it needs a real production
			// trigger, not a constructor call. This is the story's own
			// condition — a table carrying `style.align: "justify"`,
			// which cascades into cells that cannot draw it — and it
			// arrives located at the element and the field.
			source := strings.Replace(roundTripGoldenSource(t), "\"id\": \"e2\",\n          \"style\": {", "\"id\": \"e2\",\n          \"style\": {\n            \"align\": \"justify\",", 1)
			if source == roundTripGoldenSource(t) {
				t.Fatal("fixture precondition: the table element's style block was not found, so this trigger would exercise nothing")
			}
			_, err := ParseTemplate([]byte(source))
			return err
		},
		diag.CodeExpressionInvalid: func(t *testing.T) error {
			_, err := ParseTemplate([]byte(invalidExpressionTemplateJSON))
			return err
		},
		diag.CodeBindingPathAbsent: func(t *testing.T) error {
			tpl, err := ParseTemplate([]byte(unresolvableBindingTemplateJSON))
			if err != nil {
				return err
			}
			_, err = Render(tpl, Data(`{}`), Params(`{}`), testShippedFontSet())
			return err
		},
		diag.CodeContentUnlayoutable: func(t *testing.T) error {
			tpl, err := ParseTemplate([]byte(overflowLineTemplate))
			if err != nil {
				return err
			}
			_, err = Render(tpl, Data(`{}`), Params(`{}`), testShippedFontSet())
			return err
		},
		diag.CodeDocumentDateInvalid: func(t *testing.T) error {
			tpl, err := ParseTemplate([]byte(unresolvableBindingTemplateJSON))
			if err != nil {
				return err
			}
			_, err = Render(tpl, Data(`{"customer":{"name":"Ada"}}`), Params(`{"documentDate":"not-rfc3339"}`), testShippedFontSet())
			return err
		},
		diag.CodeTableFooterSourceUnresolved: func(t *testing.T) error {
			source := roundTripGoldenSource(t)
			source = strings.Replace(source, `{{formatNumber(transaction.amount, \"#,##0.00\")}}`, `{{transaction}}`, 1)
			_, err := ParseTemplate([]byte(source))
			return err
		},
		diag.CodeTableMinHeightUnplaceable: func(t *testing.T) error {
			// SPEC-table-rules §3. A LOAD-time trigger: the floor, the page size,
			// the margins and the band heights are all declared, so the
			// condition is decidable at ParseTemplate with no data.
			// 10000pt is taller than any page this format can describe.
			source := strings.Replace(roundTripGoldenSource(t), `"headerHeight":`, "\"minHeight\": 10000,\n          \"headerHeight\":", 1)
			if source == roundTripGoldenSource(t) {
				t.Fatal("fixture precondition: the table element's headerHeight was not found, so this trigger would exercise nothing")
			}
			_, err := ParseTemplate([]byte(source))
			return err
		},
		// spec-section-break CAP-5: both LOAD-time, decidable from the
		// document and its page geometry alone.
		diag.CodeSectionBreakInvalid: func(t *testing.T) error {
			_, err := ParseTemplate([]byte(sectionBreakTestDoc(`, "sectionBreak": 0`, "")))
			return err
		},
		diag.CodeSectionBreakStraddled: func(t *testing.T) error {
			// The legend's box is 80..92pt; a break at 85 runs through it.
			_, err := ParseTemplate([]byte(sectionBreakTestDoc(`, "sectionBreak": 85`, "")))
			return err
		},
		// SPEC-multi-pages CAP-7: LOAD-time — a `pages` array with fewer than
		// two entries.
		diag.CodePagesInvalid: func(t *testing.T) error {
			source := strings.Replace(roundTripGoldenSource(t), `"nextId":`, "\"pages\": [],\n  \"nextId\":", 1)
			if source == roundTripGoldenSource(t) {
				t.Fatal("fixture precondition: nextId was not found, so this trigger would exercise nothing")
			}
			_, err := ParseTemplate([]byte(source))
			return err
		},
		diag.CodeTextFaceAbsent: func(t *testing.T) error {
			// spec-deferred-offline-cache CAP-7. A REAL production
			// trigger on the public Render path: the document declares
			// the chain ["Noto Sans"] and the caller supplies a FontSet
			// that does not carry it, so the first rune of the element
			// is uncovered by a chain whose only member was never
			// supplied. Before CAP-7 this shipped a PDF with the text
			// silently gone under a TEXT_MISSING_GLYPH Warning.
			tpl, err := ParseTemplate([]byte(missingGlyphTemplateJSON))
			if err != nil {
				return err
			}
			_, err = Render(tpl, Data(`{"name":"A"}`), Params(`{}`), FontSet{"Noto Sans Thai": testShippedNotoSansThai})
			return err
		},
		diag.CodeTableFooterSourceForbidden: func(t *testing.T) error {
			source := roundTripGoldenSource(t)
			source = strings.Replace(source, `"footer": "sum",`, "\"footer\": \"count\",\n              \"footerOf\": \"transactions.amount\",", 1)
			_, err := ParseTemplate([]byte(source))
			return err
		},
	}

	expected := diag.ErrorCodes()
	if len(expected) == 0 {
		t.Fatal("registry declares no Error codes; census would be vacuous")
	}
	if len(triggers) != len(expected) {
		t.Fatalf("error trigger map has %d entries for %d registry Error codes", len(triggers), len(expected))
	}
	for _, code := range expected {
		trigger, ok := triggers[code]
		if !ok {
			t.Fatalf("registered Error code %q has no production trigger", code)
		}
		t.Run(string(code), func(t *testing.T) {
			err := trigger(t)
			if err == nil {
				t.Fatalf("production trigger for %q succeeded", code)
			}
			var renderErr *RenderError
			if !errors.As(err, &renderErr) {
				t.Fatalf("%q error is not transported as RenderError: %T %v", code, err, err)
			}
			d := renderErr.Diagnostic
			if d.Severity != SeverityError {
				t.Errorf("%q severity = %s, want Error", code, d.Severity)
			}
			if d.Code != string(code) {
				t.Errorf("code = %q, want registry code %q; production error = %v", d.Code, code, err)
			}
			if strings.TrimSpace(d.Message) == "" {
				t.Errorf("%q has blank actionable message", code)
			}
			if code != diag.CodeTemplateMalformed && code != diag.CodeDocumentDateInvalid && d.ElementID == "" && d.DataPath == "" {
				t.Errorf("%q lacks an applicable element or data-path location: %+v", code, d)
			}
		})
	}

	// Warnings must be exercised just as dynamically as errors.  The only
	// exception to a public Render result is INTERNAL_UNHANDLED_CAVEAT: its
	// production path is deliberately unreachable while expr.CaveatKind is a
	// closed set, so its real return-site is driven with a future-kind value.
	warnings := map[diag.Code]func(*testing.T) Result{
		diag.CodeTextClippedWidth: func(t *testing.T) Result { return renderClipTemplate(t, clipNarrowTemplate) },
		diag.CodeEmptyAverage: func(t *testing.T) Result {
			tpl, err := ParseTemplate([]byte(emptyAverageTemplateJSON("")))
			if err != nil {
				t.Fatal(err)
			}
			result, err := Render(tpl, Data(`{"t":[]}`), nil, testShippedFontSet())
			if err != nil {
				t.Fatal(err)
			}
			return result
		},
		diag.CodeTextMissingGlyph: func(t *testing.T) Result {
			tpl, err := ParseTemplate([]byte(missingGlyphTemplateJSON))
			if err != nil {
				t.Fatal(err)
			}
			result, err := Render(tpl, Data(`{"name":"ก"}`), Params(`{}`), testShippedFontSet())
			if err != nil {
				t.Fatal(err)
			}
			return result
		},
		diag.CodeTextStyleFaceUndeclared: func(t *testing.T) Result {
			// Story 11.2, FR57. A REAL production trigger, not a
			// constructed Diagnostic: the worked example's element `e1`
			// declares `style.bold`, and its `body` chain is three bare
			// face names that declare no bold variant — so every rune of
			// it renders in its own base face and says so.
			source := roundTripGoldenSource(t)
			if !strings.Contains(source, `"bold": true`) {
				t.Fatal("fixture precondition: worked-example.json declares no style.bold, so this trigger would exercise nothing")
			}
			// AND THE CHAIN MUST DECLARE NO VARIANT, or the trigger
			// passes for the wrong reason — a chain that HAS a bold face
			// resolves it and emits nothing. An object-form entry is the
			// only way a chain can carry one, and an object-form entry is
			// the only `{` inside the fonts block.
			// GUARD THE SLICE, not only the fixture. This trigger checks
			// a precondition ABOUT worked-example.json's content and then
			// parses that content by hand; an unguarded strings.Index
			// would index from -1 and panic, taking the whole test binary
			// down and hiding every other census failure with it.
			open := strings.Index(source, `"fonts": {`)
			if open < 0 {
				t.Fatal("fixture precondition: worked-example.json has no `\"fonts\": {` block, so this trigger cannot check that the chain declares no variant")
			}
			fontsBlock := source[open+len(`"fonts": {`):]
			close := strings.Index(fontsBlock, "\n  \"")
			if close < 0 {
				t.Fatal("fixture precondition: worked-example.json's fonts block is not followed by another top-level key, so its extent cannot be bounded")
			}
			fontsBlock = fontsBlock[:close]
			if strings.Contains(fontsBlock, "{") {
				t.Fatalf("fixture precondition: worked-example.json's fonts block now carries an object-form entry, so the absence arm may not be reached:\n%s", fontsBlock)
			}
			tpl, err := ParseTemplate([]byte(source))
			if err != nil {
				t.Fatal(err)
			}
			result, err := Render(tpl, Data(`{"customer":{"name":"Ada"},"transactions":[{"date":"2026-08-29","amount":1}]}`), Params(`{}`), testShippedFontSet())
			if err != nil {
				t.Fatal(err)
			}
			return result
		},
		diag.CodeInternalUnhandledCaveat: func(t *testing.T) Result {
			return Result{Bytes: []byte("mapped"), Diagnostics: []Diagnostic{diagnosticFromCaveat("e1", expr.Caveat{Kind: expr.CaveatKind(255), Path: "future.path"})}}
		},
		diag.CodeTableHeaderRepeatSuppressed: func(t *testing.T) Result {
			tpl, err := ParseTemplate([]byte(tallRowRepeatDoc()))
			if err != nil {
				t.Fatal(err)
			}
			result, err := Render(tpl, Data(multiRowTableData(3, -1)), nil, testShippedFontSet())
			if err != nil {
				t.Fatal(err)
			}
			return result
		},
		diag.CodeTableFooterOrphanSuppressed: func(t *testing.T) Result {
			return renderFooterFixture(t, footerFixtureDocUnsatisfiableTie(), footerFixtureDataUnsatisfiableTie())
		},
		// spec-barcode-qr-elements CAP-4: real renders whose data or box
		// defeats the barcode, completing with the Warning.
		diag.CodeBarcodeUnencodable: func(t *testing.T) Result {
			return renderBarcodeWitness(t, "{{ref}}", "300", `{"ref":"ก"}`)
		},
		diag.CodeBarcodeModuleTooSmall: func(t *testing.T) Result {
			return renderBarcodeWitness(t, "1234567890", "60", `{}`)
		},
		// The qrcode's three, on the same terms.
		diag.CodeQRCodeTooLong: func(t *testing.T) Result {
			return renderQRCodeWitness(t, "{{ref}}", "H", "300", `{"ref":"`+strings.Repeat("x", 1274)+`"}`)
		},
		diag.CodeQRCodeModuleTooSmall: func(t *testing.T) Result {
			return renderQRCodeWitness(t, "folio8", "M", "30", `{}`)
		},
		diag.CodeQRCodeDoesNotFit: func(t *testing.T) Result {
			return renderQRCodeWitness(t, "folio8", "M", "0.02", `{}`)
		},
		diag.CodeBarcodeDoesNotFit: func(t *testing.T) Result {
			return renderBarcodeWitness(t, "1234567890", "0.1", `{}`)
		},
		diag.CodeSectionBreakSplitsKeepTogether: func(t *testing.T) Result {
			signature := `,
      {"id": "e6", "type": "text", "x": 0, "y": 60, "width": 180, "height": 10, "value": "Signed", "keepTogether": "sig", "style": {"fontFamily": "latin", "fontSize": 8}}`
			doc := strings.Replace(sectionBreakTestDoc(sectionBreakAt75, signature), `"value": "Legend",`, `"value": "Legend", "keepTogether": "sig",`, 1)
			return sectionBreakRender(t, doc, 1)
		},
		diag.CodeTableRowClippedHeight: func(t *testing.T) Result {
			tpl, err := ParseTemplate([]byte(overTallRowDoc()))
			if err != nil {
				t.Fatal(err)
			}
			result, err := Render(tpl, Data(overTallRowFixtureData()), nil, testShippedFontSet())
			if err != nil {
				t.Fatal(err)
			}
			return result
		},
	}
	warningCount := 0
	for _, code := range diag.All() {
		disposition, _ := diag.Classified(code)
		if disposition != diag.DispositionWarning {
			continue
		}
		warningCount++
		trigger, ok := warnings[code]
		if !ok {
			t.Fatalf("registered Warning code %q has no actionable witness", code)
		}
		t.Run(string(code), func(t *testing.T) {
			result := trigger(t)
			if len(result.Bytes) == 0 {
				t.Fatalf("Warning %q discarded successful render bytes", code)
			}
			for _, d := range result.Diagnostics {
				if d.Code != string(code) {
					continue
				}
				if d.Severity != SeverityWarning {
					t.Fatalf("Warning %q severity = %s", code, d.Severity)
				}
				if d.ElementID == "" && d.DataPath == "" {
					t.Fatalf("Warning %q lacks a location: %+v", code, d)
				}
				if strings.TrimSpace(d.Message) == "" {
					t.Fatalf("Warning %q has no actionable message", code)
				}
				return
			}
			t.Fatalf("successful production witness did not return Warning %q: %+v", code, result.Diagnostics)
		})
	}
	if len(warnings) != warningCount {
		t.Fatalf("warning trigger map has %d entries for %d registry Warning codes", len(warnings), warningCount)
	}
}

func roundTripGoldenSource(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatalf("read golden source: %v", err)
	}
	return string(b)
}
