package folio8

import (
	"fmt"

	"github.com/panitw/folio8/folio8-go/internal/designer"
	"github.com/panitw/folio8/folio8-go/internal/geom"
	"github.com/panitw/folio8/folio8-go/internal/template"
)

func authoredProperty[T any](value template.Presence[T], parentNull bool) designer.AuthoredProperty[T] {
	if parentNull || value.Set && value.Null {
		return designer.AuthoredProperty[T]{State: "null"}
	}
	if !value.Set {
		return designer.AuthoredProperty[T]{State: "absent"}
	}
	return designer.AuthoredProperty[T]{State: "value", Value: &value.Value}
}

func canvasAuthoredProperties(element template.Element) (*designer.CanvasAuthoredProperties, error) {
	style := element.Style.Value
	border := style.Border.Value
	styleNull := element.Style.Set && element.Style.Null
	borderNull := styleNull || style.Border.Set && style.Border.Null
	// Empty edges are a authored value, not a nil array on the wire.
	if border.Edges.Set && !border.Edges.Null && border.Edges.Value == nil {
		border.Edges.Value = []string{}
	}
	result := &designer.CanvasAuthoredProperties{
		VisibleIf:   authoredProperty(element.VisibleIf, false),
		FontFamily:  authoredProperty(style.FontFamily, styleNull),
		FontSize:    authoredProperty(style.FontSize, styleNull),
		LineSpacing: authoredProperty(style.LineSpacing, styleNull),
		Bold:        authoredProperty(style.Bold, styleNull),
		Italic:      authoredProperty(style.Italic, styleNull),
		Align:       authoredProperty(style.Align, styleNull),
		Valign:      authoredProperty(style.Valign, styleNull),
		Color:       authoredProperty(style.Color, styleNull),
		Background:  authoredProperty(style.Background, styleNull),
		BorderWidth: authoredProperty(border.Width, borderNull),
		BorderColor: authoredProperty(border.Color, borderNull),
		BorderEdges: authoredProperty(border.Edges, borderNull),
		// The loader admits the key on a qrcode only, so every other kind
		// projects absent here.
		ErrorCorrection: authoredProperty(element.ErrorCorrection, false),
	}
	if element.Type != template.ElementText && element.Type != template.ElementTable {
		result.FontFamily = designer.AuthoredProperty[string]{State: "absent"}
		result.FontSize = designer.AuthoredProperty[geom.Length]{State: "absent"}
		result.LineSpacing = designer.AuthoredProperty[int64]{State: "absent"}
		result.Bold = designer.AuthoredProperty[bool]{State: "absent"}
		result.Italic = designer.AuthoredProperty[bool]{State: "absent"}
		result.Align = designer.AuthoredProperty[string]{State: "absent"}
		result.Valign = designer.AuthoredProperty[string]{State: "absent"}
		result.Color = designer.AuthoredProperty[string]{State: "absent"}
	}
	for _, field := range []designer.AuthoredProperty[string]{result.VisibleIf, result.FontFamily, result.Align, result.Valign, result.Color, result.Background, result.BorderColor, result.ErrorCorrection} {
		if field.Value != nil && len(*field.Value) > maxCanvasPropertyString {
			return nil, fmt.Errorf("folio8: authored property exceeds projection bound")
		}
	}
	for _, field := range []designer.AuthoredProperty[geom.Length]{result.FontSize, result.BorderWidth} {
		if field.Value != nil && (*field.Value < -geom.Length(designer.MaxCanvasMillipoints) || *field.Value > geom.Length(designer.MaxCanvasMillipoints)) {
			return nil, fmt.Errorf("folio8: authored property exceeds safe geometry")
		}
	}
	return result, nil
}
