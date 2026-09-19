// This file is the render obligation of the two code elements
// (spec-barcode-qr-elements CAP-1 to CAP-4): a barcode's or qrcode's bindable
// value is resolved through the same bind path a text value uses, encoded by
// internal/barcode (Code 128 or QR Code), fitted into its box in whole
// millipoints, and emitted as black filled page-model rectangles.
//
// The rectangles travel as a tableRectSource — the carrier element_box.go
// already uses — so pagination, keepTogether, visibility and header/footer
// repetition apply through the element id with no new machinery. A document
// without a code element contributes nothing here and its bytes are unchanged.
package folio8

import (
	"errors"
	"fmt"

	"github.com/panitw/folio8/folio-go/internal/barcode"
	"github.com/panitw/folio8/folio-go/internal/bind"
	"github.com/panitw/folio8/folio-go/internal/expr"
	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/layout"
	"github.com/panitw/folio8/folio-go/internal/pagemodel"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// isCodeElement reports whether an element type is one of the code elements
// whose value is encoded and drawn as filled rectangles.
func isCodeElement(t template.ElementType) bool {
	return t == template.ElementBarcode || t == template.ElementQRCode
}

// barcodeLayout is one barcode's resolved geometry, box-relative.
type barcodeLayout struct {
	bars        []barcode.Bar
	moduleWidth geom.Length
}

// layoutBarcode is THE ONE derivation of a barcode's bars from its resolved
// content and box, shared by the render path and the canvas paint so the two
// cannot disagree (AD-17).
//
// It returns the layout (empty when nothing is drawn) and at most one
// Warning. Empty content draws nothing and says nothing: a null or empty
// binding is the ordinary "no code for this record" case.
func layoutBarcode(elementID, content string, boxW, boxH geom.Length) (barcodeLayout, *Diagnostic) {
	if content == "" {
		return barcodeLayout{}, nil
	}
	values, err := barcode.Encode(content)
	if err != nil {
		var ue *barcode.UnencodableError
		if errors.As(err, &ue) {
			return barcodeLayout{}, &Diagnostic{
				Severity:  SeverityWarning,
				Code:      DiagCodeBarcodeUnencodable,
				ElementID: elementID,
				Message:   fmt.Sprintf("element %s: barcode not drawn: its value %s", elementID, ue.Error()),
			}
		}
		// Unreachable for non-empty content: UnencodableError is Encode's
		// only failure. Reported under the same code rather than dropped.
		return barcodeLayout{}, &Diagnostic{
			Severity:  SeverityWarning,
			Code:      DiagCodeBarcodeUnencodable,
			ElementID: elementID,
			Message:   fmt.Sprintf("element %s: barcode not drawn: %v", elementID, err),
		}
	}
	runs := barcode.Runs(values)
	fit, ok := barcode.FitWidth(runs, boxW)
	if !ok || boxH <= 0 {
		return barcodeLayout{}, &Diagnostic{
			Severity:  SeverityWarning,
			Code:      DiagCodeBarcodeDoesNotFit,
			ElementID: elementID,
			Message: fmt.Sprintf("element %s: barcode not drawn: %d modules plus %d quiet-zone modules on each side need at least %d mp of width and a positive height, and the box is %d x %d mp — widen the box or shorten the value",
				elementID, barcode.Modules(runs), barcode.QuietZoneModules, barcode.Modules(runs)+2*barcode.QuietZoneModules, boxW, boxH),
		}
	}
	out := barcodeLayout{bars: barcode.Bars(runs, fit), moduleWidth: fit.ModuleWidth}
	if fit.ModuleWidth < barcode.MinModuleWidth {
		return out, &Diagnostic{
			Severity:  SeverityWarning,
			Code:      DiagCodeBarcodeModuleTooSmall,
			ElementID: elementID,
			Message: fmt.Sprintf("element %s: barcode modules are %d mp wide, below the 0.25 mm (%d mp) scanners need — widen the box or shorten the value",
				elementID, fit.ModuleWidth, barcode.MinModuleWidth),
		}
	}
	return out, nil
}

// qrcodeLayout is one QR code's resolved geometry, box-relative.
type qrcodeLayout struct {
	rects       []barcode.QRRect
	moduleWidth geom.Length
}

// qrLevelOf is a qrcode element's error-correction level: the authored
// `errorCorrection`, or M when the key is absent. The loader has already
// refused any other value.
func qrLevelOf(element template.Element) barcode.ECLevel {
	if element.ErrorCorrection.Set && !element.ErrorCorrection.Null {
		if level, ok := barcode.ParseECLevel(element.ErrorCorrection.Value); ok {
			return level
		}
	}
	level, _ := barcode.ParseECLevel(template.QRErrorCorrectionDefault)
	return level
}

// layoutQRCode is THE ONE derivation of a QR code's modules from its
// resolved content, level and box, shared by the render path and the canvas
// paint (AD-17). Like layoutBarcode it returns at most one Warning, and
// empty content draws nothing and says nothing.
func layoutQRCode(elementID, content string, level barcode.ECLevel, boxW, boxH geom.Length) (qrcodeLayout, *Diagnostic) {
	if content == "" {
		return qrcodeLayout{}, nil
	}
	sym, err := barcode.EncodeQR([]byte(content), level)
	if err != nil {
		message := fmt.Sprintf("element %s: QR code not drawn: %v", elementID, err)
		var tl *barcode.QRTooLongError
		if errors.As(err, &tl) {
			message = fmt.Sprintf("element %s: QR code not drawn: its value is %s — shorten the value or lower the error-correction level", elementID, tl.Error())
		}
		return qrcodeLayout{}, &Diagnostic{
			Severity:  SeverityWarning,
			Code:      DiagCodeQRCodeTooLong,
			ElementID: elementID,
			Message:   message,
		}
	}
	fit, ok := barcode.FitQR(sym.Size, boxW, boxH)
	if !ok {
		total := sym.Size + 2*barcode.QRQuietZoneModules
		return qrcodeLayout{}, &Diagnostic{
			Severity:  SeverityWarning,
			Code:      DiagCodeQRCodeDoesNotFit,
			ElementID: elementID,
			Message: fmt.Sprintf("element %s: QR code not drawn: a version-%d symbol is %d modules plus %d quiet-zone modules on each side, needing a square of at least %d mp, and the box is %d x %d mp — enlarge the box or shorten the value",
				elementID, sym.Version, sym.Size, barcode.QRQuietZoneModules, total, boxW, boxH),
		}
	}
	out := qrcodeLayout{rects: barcode.QRRects(sym, fit), moduleWidth: fit.ModuleWidth}
	if fit.ModuleWidth < barcode.QRMinModuleWidth {
		return out, &Diagnostic{
			Severity:  SeverityWarning,
			Code:      DiagCodeQRCodeModuleTooSmall,
			ElementID: elementID,
			Message: fmt.Sprintf("element %s: QR code modules are %d mp wide, below the 0.5 mm (%d mp) scanners need — enlarge the box, shorten the value or lower the error-correction level",
				elementID, fit.ModuleWidth, barcode.QRMinModuleWidth),
		}
	}
	return out, nil
}

// codeRect is one filled rectangle of a code element, box-relative.
type codeRect struct {
	X, Y, W, H geom.Length
}

// codeLayout is either code element's drawn rectangles and module width.
type codeLayout struct {
	rects       []codeRect
	moduleWidth geom.Length
}

// layoutCode is the per-kind layout function the shared band walk
// (collectBarcodeRects) and the placement check (canvasBarcodeIsPlaced) call:
// a barcode's bars span the box height; a QR code's rects are its merged
// module runs. The canvas paint calls layoutBarcode and layoutQRCode directly,
// because its two paint shapes differ.
func layoutCode(element template.Element, content string, boxW, boxH geom.Length) (codeLayout, *Diagnostic) {
	if element.Type == template.ElementQRCode {
		lay, warning := layoutQRCode(string(element.ID), content, qrLevelOf(element), boxW, boxH)
		out := codeLayout{moduleWidth: lay.moduleWidth, rects: make([]codeRect, 0, len(lay.rects))}
		for _, r := range lay.rects {
			out.rects = append(out.rects, codeRect{X: r.X, Y: r.Y, W: r.W, H: r.H})
		}
		return out, warning
	}
	lay, warning := layoutBarcode(string(element.ID), content, boxW, boxH)
	out := codeLayout{moduleWidth: lay.moduleWidth, rects: make([]codeRect, 0, len(lay.bars))}
	for _, bar := range lay.bars {
		out.rects = append(out.rects, codeRect{X: bar.X, W: bar.W, H: boxH})
	}
	return out, warning
}

// collectBarcodeRects walks every band's code elements — barcodes and QR
// codes — in document order. It returns one tableRectSource per visible code
// that draws, and each band's code Warnings, indexed like bands.
//
// Binding runs for EVERY code element, visible or not
// (render_visibility.go's R2): an absent path is a located Error whichever
// report the template is handed. Only a hidden code's output and Warnings
// are suppressed.
func collectBarcodeRects(doc *Template, bands []bandWithOrigin, data, params bind.Value, visible visibilityVerdicts) ([]tableRectSource, [][]Diagnostic, error) {
	fc := expr.NewFormatContext(doc.doc.Locale, doc.doc.UTCOffset)
	var sources []tableRectSource
	diags := make([][]Diagnostic, len(bands))
	for bandIndex, b := range bands {
		for _, el := range b.band.Elements {
			if !isCodeElement(el.Type) {
				continue
			}
			if !el.Value.Set || el.Value.Null || el.Value.Value == "" {
				continue
			}
			content, _, _, berr := bind.BindTextSpans(el.Value.Value, data, params, fc, string(el.ID))
			if berr != nil {
				return nil, nil, expressionRuntimeError(string(el.ID), "value", fmt.Errorf("folio8: Render: %w", berr))
			}
			if !isVisible(visible, el.ID) {
				continue
			}
			w, h := el.Width.Value, el.Height.Value
			if !el.Width.Set || el.Width.Null || !el.Height.Set || el.Height.Null {
				w, h = 0, 0
			}
			lay, warning := layoutCode(el, content, w, h)
			if warning != nil {
				diags[bandIndex] = append(diags[bandIndex], *warning)
			}
			if len(lay.rects) == 0 {
				continue
			}
			top := layout.PlaceInBand(b.origin, el.Y)
			rects := make([]pagemodel.Rect, 0, len(lay.rects))
			for _, r := range lay.rects {
				rects = append(rects, pagemodel.Rect{
					X: el.X + r.X, Y: top + r.Y, W: r.W, H: r.H,
					HasFill: true, Fill: pagemodel.Color{R: 0, G: 0, B: 0},
				})
			}
			sources = append(sources, tableRectSource{
				band:      bandIndex,
				elementID: string(el.ID),
				top:       top,
				bottom:    top + h,
				rects:     rects,
			})
		}
	}
	return sources, diags, nil
}
