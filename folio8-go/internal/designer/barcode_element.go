package designer

// CanvasBarcodeBar is one bar, in millipoints, its X relative to the
// component's own left edge. Every bar spans the component's full height.
type CanvasBarcodeBar struct {
	X     int64 `json:"x"`
	Width int64 `json:"width"`
}

// CanvasBarcodePaint is the Go-computed geometry the canvas draws for a
// barcode: the same bars layoutBarcode gives the render path.
type CanvasBarcodePaint struct {
	ModuleWidth int64              `json:"moduleWidth"`
	Bars        []CanvasBarcodeBar `json:"bars"`
}

// CanvasQRCodeRect is one horizontal run of dark modules, in millipoints,
// relative to the component's top-left corner.
type CanvasQRCodeRect struct {
	X      int64 `json:"x"`
	Y      int64 `json:"y"`
	Width  int64 `json:"width"`
	Height int64 `json:"height"`
}

// CanvasQRCodePaint is the Go-computed geometry the canvas draws for a
// qrcode: the same rects layoutQRCode gives the render path, rows top to
// bottom and left to right within a row.
type CanvasQRCodePaint struct {
	ModuleWidth int64              `json:"moduleWidth"`
	Rects       []CanvasQRCodeRect `json:"rects"`
}
