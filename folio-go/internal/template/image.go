package template

import (
	"fmt"
)

// This file decodes JPEG and PNG bytes ENOUGH to (a) validate that a
// RECOGNISED mediaType's bytes really are that format (AC9, AC11b — a
// reader-independent, "the file lies about itself" check) and (b)
// produce the passthrough PDF embedding shape (AC9, AC10, D-1.8.1): the
// already-compressed bytes are carried into the PDF unchanged, never
// decoded to pixels and never re-encoded. No stdlib or third-party
// image/compression package is imported anywhere in this module (AC10) —
// this is a chunk/segment walker, not a decoder.
//
// Two mediaType strings are RECOGNISED: "image/png" and "image/jpeg".
// Every other mediaType is OPEN — never inspected at load time (D-1.8.1
// amended, AC11/AC11a) — and only becomes a located error at RENDER time,
// only when an element actually needs to draw it (DecodeImageForRender
// below).

// maxImagePixelDimension is AC15's arithmetic-defended bound: ScaleRound
// panics when v*num overflows int64, and the fit computation's v*num is
// bw*H (or bh*W) — bw/bh are page-box millipoints, always under 10^6 for
// any page size this module can produce. A pixel bound of 10^6 keeps
// bw*H under 10^12, four orders of magnitude clear of int64's ~9.2*10^18
// ceiling, with ample headroom for both operands of the cross-multiply
// comparison in AC13.
const maxImagePixelDimension = 1_000_000

// DecodedImage is a recognised, validated, passthrough-ready image: its
// intrinsic pixel dimensions (validated, unexported — AC15a, D-1.5.2's
// pattern: "the validated value is the only one reachable"; there is no
// accessor to a raw, unvalidated header field) and everything
// internal/pdf needs to embed the ALREADY-COMPRESSED bytes verbatim as
// an XObject stream (D-1.8.1's passthrough design; no compressor is ever
// invoked, AC10).
type DecodedImage struct {
	width, height int64 // pixel counts, validated: 0 < w,h <= maxImagePixelDimension — plain int64, NOT geom.Length: these are pixel counts, not millipoints (AC7: routed through appendInt, never appendLength)

	// Filter is the PDF stream filter name, e.g. "DCTDecode" or
	// "FlateDecode" (without the leading slash — internal/pdf owns PDF
	// name spelling).
	Filter string

	// HasDecodeParms reports whether DecodeParms fields below apply
	// (PNG only; JPEG passthrough carries no /DecodeParms).
	HasDecodeParms   bool
	PredictorColors  int64 // /Colors
	PredictorBPC     int64 // /BitsPerComponent
	PredictorColumns int64 // /Columns (== pixel width)

	// ColorSpace is the PDF /ColorSpace name, without the leading slash.
	ColorSpace string

	BitsPerComponent int64

	// Stream is the exact bytes to place inside the XObject's stream
	// body: for JPEG, the whole file's bytes unchanged; for PNG, every
	// IDAT chunk's data concatenated in order — the file's own zlib
	// stream, passed through (D-1.8.1, AC10a: "the bytes are the file's
	// own IDAT stream", never a compressor's output; substituting a
	// re-encode is a hash-changing versioned event under AD-22). Carries
	// risk R4 ("compressor output is stable by observation, not by
	// contract" — acceptance.md:83): D-1.8.1's passthrough design exists
	// specifically to keep R4 closed, by never invoking a compressor on
	// this path at all.
	Stream []byte
}

// Width and Height return the validated pixel dimensions (AC15a: the
// only way to observe them — there is no path to an unvalidated raw
// header value).
func (d DecodedImage) Width() int64  { return d.width }
func (d DecodedImage) Height() int64 { return d.height }

// UnsupportedMediaTypeError is AC11a's "library capability" failure: the
// document is valid (mediaType stays an open set, D-1.4.12), but THIS
// version of the library cannot render this media type. It is never
// raised by the loader — only by DecodeImageForRender, called from the
// render path (AC11a's "one named predicate, one call site" — the
// loader never calls this function).
type UnsupportedMediaTypeError struct {
	AssetKey  string
	ElementID string
	MediaType string
}

func (e *UnsupportedMediaTypeError) Error() string {
	return fmt.Sprintf(
		"template: element %s: asset %s: this version cannot render media type %q "+
			"(the document is valid — mediaType is an open set, D-1.4.12; this is a library "+
			"capability limit, not a format error)",
		e.ElementID, e.AssetKey, e.MediaType,
	)
}

// recognisedMediaTypes is the CLOSED set of media types this function
// itself understands well enough to decode. It is deliberately NOT
// internal/template's closedsets.go: AC11 forbids mediaType from ever
// joining that file's closed-set inventory (a closed set there can only
// be extended by a MAJOR bump, D-1.4.12) — this set instead governs only
// "can THIS library version draw it", which is free to grow release to
// release without being a breaking format change.
func decodeRecognisedImage(mediaType string, data []byte) (DecodedImage, bool, error) {
	switch mediaType {
	case "image/png":
		img, err := decodePNG(data)
		return img, true, err
	case "image/jpeg":
		img, err := decodeJPEG(data)
		return img, true, err
	default:
		return DecodedImage{}, false, nil
	}
}

// DecodeImageForRender is AC11a's one named predicate: never called from
// LoadTemplate/ParseTemplate. It has two call sites — the render path, for
// an asset an element actually needs to draw, and Story 5.13's
// setComponentAsset authoring command (folio-go/component_commands.go),
// which reuses it as the command's own media-type-recognition and
// decode-validation check rather than restating decodeRecognisedImage's
// rules a second time or leaning on decodeAssets as the catcher. assetKey
// and elementID identify the element and asset for both the capability
// error and any (already load-time-proven impossible in practice for a
// loaded document, but handled rather than assumed) format error.
func DecodeImageForRender(mediaType string, data []byte, assetKey, elementID string) (DecodedImage, error) {
	img, recognised, err := decodeRecognisedImage(mediaType, data)
	if !recognised {
		return DecodedImage{}, &UnsupportedMediaTypeError{AssetKey: assetKey, ElementID: elementID, MediaType: mediaType}
	}
	if err != nil {
		return DecodedImage{}, newLoadError("assets."+assetKey, elementID, mediaType, err.Error())
	}
	return img, nil
}

// validateImageDimensions is AC15/AC15a's trust-boundary check, shared
// by both formats: W and H must be positive and bounded BEFORE either
// ever reaches geom.ScaleRound (D-1.5.2's precedent, applied one story
// later — ScaleRound's panics are programmer-error guards, not input
// handling, so a corrupt header must produce a located load error here,
// never a panic downstream).
func validateImageDimensions(w, h int64) (int64, int64, error) {
	if w <= 0 {
		return 0, 0, fmt.Errorf("image width is %d, must be > 0", w)
	}
	if h <= 0 {
		return 0, 0, fmt.Errorf("image height is %d, must be > 0", h)
	}
	if w > maxImagePixelDimension {
		return 0, 0, fmt.Errorf("image width %d exceeds the supported bound of %d pixels", w, maxImagePixelDimension)
	}
	if h > maxImagePixelDimension {
		return 0, 0, fmt.Errorf("image height %d exceeds the supported bound of %d pixels", h, maxImagePixelDimension)
	}
	return w, h, nil
}

// --- PNG ---

var pngSignature = [8]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

// decodePNG walks a PNG's chunk structure directly (no image/png, no
// compress/zlib — AC10) and returns a passthrough-ready DecodedImage, or
// an error naming exactly what is wrong: signature, truncation, an
// unsupported IHDR (alpha, palette, 16-bit or interlaced — AC9's
// "recognised but the bytes are read and rejected on their own
// content"), or a missing/malformed IDAT/IEND.
func decodePNG(data []byte) (DecodedImage, error) {
	if len(data) < 8 || [8]byte(data[:8]) != pngSignature {
		return DecodedImage{}, fmt.Errorf("not a valid PNG: bad signature")
	}
	pos := 8

	var (
		haveIHDR            bool
		width, height       int64
		bitDepth, colorType int
		interlace           int
		idat                []byte
		haveIEND            bool
	)

	for pos < len(data) {
		if len(data)-pos < 8 {
			return DecodedImage{}, fmt.Errorf("truncated PNG: incomplete chunk header at offset %d", pos)
		}
		length := be32(data[pos : pos+4])
		typ := string(data[pos+4 : pos+8])
		pos += 8
		if len(data)-pos < int(length)+4 {
			// Compared entirely in int (never in uint32) so that a
			// declared length with fewer than 4 bytes remaining cannot
			// underflow the comparison and wrap past the guard — see
			// D-1.5.2, Finding 1 (Story 1.8 review): the previous
			// `uint32(len(data)-pos)-4` wrapped to ~4.29e9 whenever
			// `len(data)-pos < 4`, letting a truncated-in-the-header PNG
			// reach the slice below and panic instead of returning a
			// located error. The +4 still reserves room for the trailing
			// CRC; this also catches the case where length alone overruns
			// the buffer.
			return DecodedImage{}, fmt.Errorf("truncated PNG: chunk %q declares length %d past end of data", typ, length)
		}
		body := data[pos : pos+int(length)]
		pos += int(length)
		pos += 4 // CRC — not verified: a CRC mismatch does not change what bytes decodePNG would emit, and the format checks above already catch truncation/malformation.

		switch typ {
		case "IHDR":
			if length != 13 {
				return DecodedImage{}, fmt.Errorf("malformed PNG: IHDR length is %d, want 13", length)
			}
			width = int64(be32(body[0:4]))
			height = int64(be32(body[4:8]))
			bitDepth = int(body[8])
			colorType = int(body[9])
			// body[10] compression method, body[11] filter method — both
			// fixed at 0 in the PNG spec; not surfaced.
			interlace = int(body[12])
			haveIHDR = true
		case "IDAT":
			idat = append(idat, body...)
		case "IEND":
			haveIEND = true
		}
	}

	if !haveIHDR {
		return DecodedImage{}, fmt.Errorf("malformed PNG: no IHDR chunk")
	}
	if !haveIEND {
		return DecodedImage{}, fmt.Errorf("malformed PNG: no IEND chunk (truncated file)")
	}
	if len(idat) == 0 {
		return DecodedImage{}, fmt.Errorf("malformed PNG: no IDAT data")
	}
	if interlace != 0 {
		return DecodedImage{}, fmt.Errorf("unsupported PNG: interlaced images are not supported (D-1.8.1: passthrough only)")
	}
	if bitDepth != 8 {
		return DecodedImage{}, fmt.Errorf("unsupported PNG: bit depth %d is not supported (only 8-bit, D-1.8.1)", bitDepth)
	}
	var colors int64
	switch colorType {
	case 0: // grayscale
		colors = 1
	case 2: // truecolor
		colors = 3
	case 3:
		return DecodedImage{}, fmt.Errorf("unsupported PNG: palette (colour type 3) is not supported (D-1.8.1: passthrough only)")
	case 4:
		return DecodedImage{}, fmt.Errorf("unsupported PNG: grayscale+alpha (colour type 4) is not supported — this library refuses transparent images (AC12, D-1.8.1)")
	case 6:
		return DecodedImage{}, fmt.Errorf("unsupported PNG: truecolor+alpha (colour type 6) is not supported — this library refuses transparent images (AC12, D-1.8.1)")
	default:
		return DecodedImage{}, fmt.Errorf("malformed PNG: unrecognised colour type %d", colorType)
	}

	w, h, err := validateImageDimensions(width, height)
	if err != nil {
		return DecodedImage{}, fmt.Errorf("PNG header: %w", err)
	}

	colorSpace := "DeviceRGB"
	if colorType == 0 {
		colorSpace = "DeviceGray"
	}

	return DecodedImage{
		width: w, height: h,
		Filter:           "FlateDecode",
		HasDecodeParms:   true,
		PredictorColors:  colors,
		PredictorBPC:     8,
		PredictorColumns: w,
		ColorSpace:       colorSpace,
		BitsPerComponent: 8,
		Stream:           idat,
	}, nil
}

func be32(b []byte) uint32 {
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// --- JPEG ---

// decodeJPEG walks a JPEG's marker segments directly (no image/jpeg —
// AC10) far enough to locate the frame header (SOF0/SOF1/SOF2 — baseline
// and progressive both passthrough identically, since neither is ever
// decoded to pixels) and read width/height/component count, then embeds
// the WHOLE file unchanged as the /DCTDecode stream body (D-1.8.1: "zero
// re-encoding, byte-stable by construction").
func decodeJPEG(data []byte) (DecodedImage, error) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return DecodedImage{}, fmt.Errorf("not a valid JPEG: missing SOI marker")
	}
	if data[len(data)-2] != 0xFF || data[len(data)-1] != 0xD9 {
		return DecodedImage{}, fmt.Errorf("malformed JPEG: missing trailing EOI marker (truncated file)")
	}

	pos := 2
	var (
		width, height int64
		components    int
		haveSOF       bool
	)
	for pos+4 <= len(data) {
		if data[pos] != 0xFF {
			return DecodedImage{}, fmt.Errorf("malformed JPEG: expected marker at offset %d", pos)
		}
		marker := data[pos+1]
		pos += 2
		if marker == 0xD8 || marker == 0xD9 || (marker >= 0xD0 && marker <= 0xD7) || marker == 0x01 {
			continue // markers with no length/payload
		}
		if pos+2 > len(data) {
			return DecodedImage{}, fmt.Errorf("truncated JPEG: incomplete segment length at offset %d", pos)
		}
		segLen := int(data[pos])<<8 | int(data[pos+1])
		if segLen < 2 || pos+segLen > len(data) {
			return DecodedImage{}, fmt.Errorf("malformed JPEG: segment at offset %d declares invalid length %d", pos, segLen)
		}
		isSOF := marker >= 0xC0 && marker <= 0xCF && marker != 0xC4 && marker != 0xC8 && marker != 0xCC
		if isSOF {
			if segLen < 8 {
				return DecodedImage{}, fmt.Errorf("malformed JPEG: SOF segment too short")
			}
			payload := data[pos+2 : pos+segLen]
			height = int64(payload[1])<<8 | int64(payload[2])
			width = int64(payload[3])<<8 | int64(payload[4])
			components = int(payload[5])
			haveSOF = true
			break
		}
		if marker == 0xDA { // start of scan: frame header must have already been found
			break
		}
		pos += segLen
	}

	if !haveSOF {
		return DecodedImage{}, fmt.Errorf("malformed JPEG: no SOF (frame header) segment found")
	}

	var colorSpace string
	switch components {
	case 1:
		colorSpace = "DeviceGray"
	case 3:
		colorSpace = "DeviceRGB"
	case 4:
		colorSpace = "DeviceCMYK"
	default:
		return DecodedImage{}, fmt.Errorf("unsupported JPEG: %d colour components is not supported", components)
	}

	w, h, err := validateImageDimensions(width, height)
	if err != nil {
		return DecodedImage{}, fmt.Errorf("JPEG frame header: %w", err)
	}

	return DecodedImage{
		width: w, height: h,
		Filter:           "DCTDecode",
		ColorSpace:       colorSpace,
		BitsPerComponent: 8,
		Stream:           data,
	}, nil
}
