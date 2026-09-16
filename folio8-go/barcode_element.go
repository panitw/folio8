// This file holds the code elements' load-time checks, the designer's escape
// projection for their value, and their canvas paint
// (spec-barcode-qr-elements CAP-4, CAP-5). Both code elements — `barcode`
// (Code 128) and `qrcode` (QR Code) — share every step here; only the
// static encodability check and the paint shape differ by kind.
//
// ESCAPES LIVE HERE, NOT IN THE ENGINE. A `.folio` file stores the real
// characters (a carriage return is the JSON escape "\r"), and the engine
// never interprets a backslash. The designer's content box is a textarea,
// which holds a line feed but turns a typed carriage return into one, so the
// canvas projects a carriage return as a NEW LINE and a line feed and a
// backslash as `\n` and `\\`; the property command turns them back into the
// characters they name. A new line is the payment payload's field separator,
// so Enter in the box is what an author types for it. Both conversions touch
// only the text OUTSIDE {{ }}: expression string literals stay escape-free.
package folio8

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/panitw/folio8/folio8-go/internal/barcode"
	"github.com/panitw/folio8/folio8-go/internal/designer"
	"github.com/panitw/folio8/folio8-go/internal/expr"
	"github.com/panitw/folio8/folio8-go/internal/template"
)

// checkCodeValue is a code element's load-time value check: every expression
// parses and checks as a text expression does, the reserved page tokens are
// refused (a code's content must be known before layout), and the static
// text outside {{ }} must be encodable. For a barcode that means ASCII only;
// for a qrcode, that the static text alone fits a version-40 symbol at the
// element's level. Either failure could never draw for any record, so it is
// a load Error rather than a Warning.
func checkCodeValue(element template.Element) error {
	value, id := element.Value.Value, element.ID
	kind := "barcode"
	if element.Type == template.ElementQRCode {
		kind = "QR code"
	}
	literal, placeholders, trailing, serr := expr.ScanPlaceholders(value)
	if serr != nil {
		return newRenderError(DiagCodeExpressionInvalid, string(id), "value", fmt.Errorf("folio8: ParseTemplate: element %s: %s", id, serr))
	}
	for _, ph := range placeholders {
		if ph.Reserved {
			return newRenderError(DiagCodeTemplateFieldInvalid, string(id), "value", fmt.Errorf(
				"folio8: ParseTemplate: element %s: a %s value cannot use {{%s}} — page numbers are resolved after layout, and a %s's content must be known before it",
				id, kind, strings.TrimSpace(ph.Inner), kind))
		}
	}
	if err := checkTextExpressions(value, id); err != nil {
		return newRenderError(DiagCodeExpressionInvalid, string(id), "value", err)
	}
	parts := make([]string, 0, len(literal)+1)
	parts = append(parts, literal...)
	parts = append(parts, trailing)
	if element.Type == template.ElementQRCode {
		static := 0
		for _, part := range parts {
			static += len(part)
		}
		level := qrLevelOf(element)
		if max := barcode.QRMaxBytes(barcode.QRMaxVersion, level); static > max {
			return newRenderError(DiagCodeTemplateFieldInvalid, string(id), "value", fmt.Errorf(
				"folio8: ParseTemplate: element %s: the QR code value's static text is %d bytes, more than the %d a version-40 QR Code holds at error-correction level %s — shorten it or lower the level",
				id, static, max, level))
		}
		return nil
	}
	for _, part := range parts {
		if off, bad := barcode.FirstUnencodable(part); bad {
			r, _ := utf8.DecodeRuneInString(part[off:])
			return newRenderError(DiagCodeTemplateFieldInvalid, string(id), "value", fmt.Errorf(
				"folio8: ParseTemplate: element %s: barcode value contains %q (U+%04X), which Code 128 cannot encode — only ASCII 0-127 may appear outside {{ }}",
				id, r, r))
		}
	}
	return nil
}

// encodeBarcodeEscapes is the designer's view of a code element's value:
// outside {{ }}, a backslash becomes `\\`, a carriage return a new line and a
// line feed `\n`. Placeholders are copied verbatim.
func encodeBarcodeEscapes(value string) string {
	literal, placeholders, trailing, err := expr.ScanPlaceholders(value)
	if err != nil {
		// Unreachable for a loaded document (the loader refuses an
		// unterminated placeholder); escape the whole text rather than guess.
		return escapeBarcodeLiteral(value)
	}
	var b strings.Builder
	for i, ph := range placeholders {
		b.WriteString(escapeBarcodeLiteral(literal[i]))
		b.WriteString("{{")
		b.WriteString(ph.Inner)
		b.WriteString("}}")
	}
	b.WriteString(escapeBarcodeLiteral(trailing))
	return b.String()
}

func escapeBarcodeLiteral(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			b.WriteString(`\\`)
		case '\r':
			b.WriteByte('\n')
		case '\n':
			b.WriteString(`\n`)
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// decodeBarcodeEscapes is encodeBarcodeEscapes' inverse, applied by the
// property command. A new line — a line feed, or a CRLF pair — is one carriage
// return, and the typed escape `\r` still names one too. Any other backslash
// sequence, or a trailing backslash, is refused: an escape encodes exactly the
// byte it names, never a guess.
func decodeBarcodeEscapes(text string) (string, error) {
	literal, placeholders, trailing, err := expr.ScanPlaceholders(text)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for i, ph := range placeholders {
		part, err := unescapeBarcodeLiteral(literal[i])
		if err != nil {
			return "", err
		}
		b.WriteString(part)
		b.WriteString("{{")
		b.WriteString(ph.Inner)
		b.WriteString("}}")
	}
	part, err := unescapeBarcodeLiteral(trailing)
	if err != nil {
		return "", err
	}
	b.WriteString(part)
	return b.String(), nil
}

func unescapeBarcodeLiteral(s string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\r' && i+1 < len(s) && s[i+1] == '\n' {
			continue
		}
		if s[i] == '\n' {
			b.WriteByte('\r')
			continue
		}
		if s[i] != '\\' {
			b.WriteByte(s[i])
			continue
		}
		if i+1 == len(s) {
			return "", fmt.Errorf(`a trailing backslash escapes nothing; write \\ for a backslash`)
		}
		i++
		switch s[i] {
		case '\\':
			b.WriteByte('\\')
		case 'r':
			b.WriteByte('\r')
		case 'n':
			b.WriteByte('\n')
		default:
			return "", fmt.Errorf(`\%c is not a supported escape; use \r, \n or \\`, s[i])
		}
	}
	return b.String(), nil
}

// BarcodeUnavailable's bounded values, set only when Barcode is absent for a
// barcode with a non-empty value.
const (
	barcodeUnavailableUnencodable = "unencodable"
	barcodeUnavailableDoesNotFit  = "doesNotFit"
)

// QRCodeUnavailable's bounded values, set only when QRCode is absent for a
// qrcode with a non-empty value.
const (
	qrcodeUnavailableTooLong    = "tooLong"
	qrcodeUnavailableDoesNotFit = "doesNotFit"
)

// illustrativeBarcodePlaceholder stands in for every {{ }} placeholder on the
// canvas, which never sees data (owner decision, review pass 1).
const illustrativeBarcodePlaceholder = "0123456789"

// illustrativeBarcodeContent is what the canvas encodes: the value with each
// placeholder replaced by illustrativeBarcodePlaceholder. A static value is
// returned unchanged, so its canvas geometry is the PDF's.
func illustrativeBarcodeContent(value string) string {
	literal, placeholders, trailing, err := expr.ScanPlaceholders(value)
	if err != nil || len(placeholders) == 0 {
		return value
	}
	var b strings.Builder
	for i := range placeholders {
		b.WriteString(literal[i])
		b.WriteString(illustrativeBarcodePlaceholder)
	}
	b.WriteString(trailing)
	return b.String()
}

// addCanvasBarcodePaint is the code elements' paint producer, beside
// addCanvasImagePaint. A static value is encoded as written; a bound value
// draws illustrative geometry (illustrativeBarcodeContent) — the preview
// shows the real code. A code that cannot be painted degrades to a bounded
// reason and never fails the projection.
func addCanvasBarcodePaint(t *Template, projection *designer.CanvasProjection) error {
	components := make(map[string]*designer.CanvasComponent, len(projection.Components))
	for i := range projection.Components {
		component := &projection.Components[i]
		components[component.ID] = component
	}
	for _, band := range []struct {
		name     string
		elements []template.Element
	}{
		{bandPageHeader, t.doc.Bands.PageHeader.Elements},
		{bandContent, contentElements(t)},
		{bandPageFooter, t.doc.Bands.PageFooter.Elements},
	} {
		for _, element := range band.elements {
			if !isCodeElement(element.Type) {
				continue
			}
			component := components[string(element.ID)]
			if component == nil || component.Band != band.name {
				return fmt.Errorf("folio8: canvas %s component %q is missing from geometry projection", element.Type, element.ID)
			}
			if !element.Value.Set || element.Value.Null || element.Value.Value == "" {
				continue
			}
			content := illustrativeBarcodeContent(element.Value.Value)
			if element.Type == template.ElementQRCode {
				lay, warning := layoutQRCode(string(element.ID), content, qrLevelOf(element), element.Width.Value, element.Height.Value)
				if len(lay.rects) == 0 {
					if warning != nil {
						reason := qrcodeUnavailableDoesNotFit
						if warning.Code == DiagCodeQRCodeTooLong {
							reason = qrcodeUnavailableTooLong
						}
						component.QRCodeUnavailable = &reason
					}
					continue
				}
				paint := &designer.CanvasQRCodePaint{ModuleWidth: int64(lay.moduleWidth), Rects: make([]designer.CanvasQRCodeRect, 0, len(lay.rects))}
				for _, r := range lay.rects {
					paint.Rects = append(paint.Rects, designer.CanvasQRCodeRect{X: int64(r.X), Y: int64(r.Y), Width: int64(r.W), Height: int64(r.H)})
				}
				component.QRCode = paint
				continue
			}
			lay, warning := layoutBarcode(string(element.ID), content, element.Width.Value, element.Height.Value)
			if len(lay.bars) == 0 {
				if warning != nil {
					reason := barcodeUnavailableDoesNotFit
					if warning.Code == DiagCodeBarcodeUnencodable {
						reason = barcodeUnavailableUnencodable
					}
					component.BarcodeUnavailable = &reason
				}
				continue
			}
			paint := &designer.CanvasBarcodePaint{ModuleWidth: int64(lay.moduleWidth), Bars: make([]designer.CanvasBarcodeBar, 0, len(lay.bars))}
			for _, bar := range lay.bars {
				paint.Bars = append(paint.Bars, designer.CanvasBarcodeBar{X: int64(bar.X), Width: int64(bar.W)})
			}
			component.Barcode = paint
		}
	}
	return nil
}

// canvasBarcodeIsPlaced answers canvasElementIsPlaced's question for a code
// element (barcode or qrcode): the render path places a column item exactly
// when rects are drawn. A static value is decided here; a bound one is
// assumed placed, and canvasContentBandHasBoundBarcode registers that as a
// cause of inexactness.
func canvasBarcodeIsPlaced(element template.Element) bool {
	if !element.Value.Set || element.Value.Null || element.Value.Value == "" {
		return false
	}
	if stringsContainsPlaceholder(element.Value.Value) {
		return true
	}
	lay, _ := layoutCode(element, element.Value.Value, element.Width.Value, element.Height.Value)
	return len(lay.rects) > 0
}

// canvasContentBandHasBoundBarcode is a cause of window-count inexactness:
// whether a bound code element draws depends on data the canvas does not
// have.
func canvasContentBandHasBoundBarcode(t *Template) bool {
	for _, element := range contentElements(t) {
		if isCodeElement(element.Type) && element.Value.Set && !element.Value.Null && stringsContainsPlaceholder(element.Value.Value) {
			return true
		}
	}
	return false
}
