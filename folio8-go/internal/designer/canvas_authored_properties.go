package designer

import "github.com/panitw/folio8/folio8-go/internal/geom"

// AuthoredProperty is bounded inspector evidence, separate from paint. It
// preserves authored absence/null and values even when a border paints no ink.
type AuthoredProperty[T any] struct {
	State string `json:"state"`
	Value *T     `json:"value,omitempty"`
}

type CanvasAuthoredProperties struct {
	VisibleIf   AuthoredProperty[string]      `json:"visibleIf"`
	FontFamily  AuthoredProperty[string]      `json:"fontFamily"`
	FontSize    AuthoredProperty[geom.Length] `json:"fontSize"`
	LineSpacing AuthoredProperty[int64]       `json:"lineSpacing"`
	Bold        AuthoredProperty[bool]        `json:"bold"`
	Italic      AuthoredProperty[bool]        `json:"italic"`
	Align       AuthoredProperty[string]      `json:"align"`
	Valign      AuthoredProperty[string]      `json:"valign"`
	Color       AuthoredProperty[string]      `json:"color"`
	Background  AuthoredProperty[string]      `json:"background"`
	BorderWidth AuthoredProperty[geom.Length] `json:"borderWidth"`
	BorderColor AuthoredProperty[string]      `json:"borderColor"`
	BorderEdges AuthoredProperty[[]string]    `json:"borderEdges"`
	// ErrorCorrection is a qrcode's authored level (absent means the default
	// M); always absent on every other kind.
	ErrorCorrection AuthoredProperty[string] `json:"errorCorrection"`
}
