package folio8

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// png3x2RGB / png2x2Alpha / jpeg4x4 mirror internal/template's own test
// fixtures of the same name and bytes (package boundaries mean they
// cannot be shared directly — same reasoning repoRootFromTest's
// duplicate comment gives).
var png3x2RGB = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x00, 0x02, 0x08, 0x02, 0x00, 0x00, 0x00, 0x12, 0x16, 0xf1,
	0x4d, 0x00, 0x00, 0x00, 0x18, 0x49, 0x44, 0x41, 0x54, 0x78, 0xda, 0x62, 0xfa, 0xcf, 0xc0, 0xc0,
	0x00, 0xc6, 0x4c, 0x10, 0xea, 0x3f, 0x03, 0x03, 0x20, 0x00, 0x00, 0xff, 0xff, 0x3c, 0x14, 0x05,
	0xff, 0xcd, 0x8e, 0xc5, 0xb9, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60,
	0x82,
}

var png2x2Alpha = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x02, 0x08, 0x06, 0x00, 0x00, 0x00, 0x72, 0xb6, 0x0d,
	0x24, 0x00, 0x00, 0x00, 0x1f, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x04, 0xc0, 0x01, 0x09, 0x00,
	0x20, 0x00, 0x04, 0xb1, 0xe1, 0x1b, 0xdc, 0xe2, 0x07, 0xee, 0xc6, 0x5b, 0x9c, 0x49, 0xf8, 0x01,
	0x00, 0x00, 0xff, 0xff, 0x2e, 0x36, 0x04, 0x81, 0xef, 0x34, 0x2a, 0x37, 0x00, 0x00, 0x00, 0x00,
	0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// imageTemplateJSON builds a `.folio` document with one image element in
// content, referencing the given asset key/bytes/mediaType, with the
// element's box given by boxW/boxH (millipoints).
func imageTemplateJSON(assetKey string, wrapped []string, mediaType string, boxW, boxH int) string {
	dataJSON := `["` + strings.Join(wrapped, `","`) + `"]`
	return `{
  "assets": {"` + assetKey + `": {"data": ` + dataJSON + `, "mediaType": "` + mediaType + `"}},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "image", "asset": "` + assetKey + `", "x": 10, "y": 20, "width": ` + itoaHelper(boxW) + `, "height": ` + itoaHelper(boxH) + `}
    ]},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}`
}

func itoaHelper(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func wrap76(b []byte) []string {
	joined := base64Std(b)
	var out []string
	for i := 0; i < len(joined); i += 76 {
		end := i + 76
		if end > len(joined) {
			end = len(joined)
		}
		out = append(out, joined[i:end])
	}
	return out
}

func base64Std(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

// TestRenderEmbedsImageXObject is source AC3: a document carrying an
// image element with an asset actually produces an image XObject in the
// rendered PDF — the baseline this story starts from is "renders as
// nothing, silently" (M-2).
func TestRenderEmbedsImageXObject(t *testing.T) {
	key := sha256Hex(png3x2RGB)
	doc := imageTemplateJSON(key, wrap76(png3x2RGB), "image/png", 100, 100)
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	outRes, err := Render(tpl, Data("{}"), nil, nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := outRes.Bytes
	if !bytes.Contains(out, []byte("/Subtype /Image")) {
		t.Fatal("rendered PDF does not contain an image XObject")
	}
	if !bytes.Contains(out, []byte("/FlateDecode")) {
		t.Fatal("expected the PNG passthrough route (/FlateDecode)")
	}
	if !bytes.Contains(out, []byte("/DecodeParms")) {
		t.Fatal("expected /DecodeParms for the PNG predictor route")
	}
}

// TestRenderEmbedsJPEGXObject exercises the JPEG passthrough route
// (/DCTDecode, whole file unchanged).
func TestRenderEmbedsJPEGXObject(t *testing.T) {
	key := sha256Hex(jpeg4x4Bytes())
	doc := imageTemplateJSON(key, wrap76(jpeg4x4Bytes()), "image/jpeg", 100, 100)
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	outRes, err := Render(tpl, Data("{}"), nil, nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := outRes.Bytes
	if !bytes.Contains(out, []byte("/DCTDecode")) {
		t.Fatal("expected the JPEG passthrough route (/DCTDecode)")
	}
	if bytes.Contains(out, []byte("/DecodeParms")) {
		t.Fatal("JPEG passthrough must not carry /DecodeParms")
	}
}

// TestResolveImagePlacementNeverStretchesNeverCrops is AC16: the drawn
// dw:dh ratio matches the intrinsic W:H by cross-multiplication, and the
// drawn box never exceeds the declared box.
func TestResolveImagePlacementNeverStretchesNeverCrops(t *testing.T) {
	// 3x2 image (from png3x2RGB) inside a SQUARE box: width binds
	// (image is wider relative to its height than the box is).
	run := imageRunSource{elementID: "e1", assetKey: "k", x: 0, y: 0, boxW: 90000, boxH: 90000}
	img, err := template.DecodeImageForRender("image/png", png3x2RGB, "k", "e1")
	if err != nil {
		t.Fatalf("DecodeImageForRender: %v", err)
	}
	_, _, dw, dh := resolveImagePlacement(run, img)
	if dw > run.boxW || dh > run.boxH {
		t.Fatalf("drawn box (%d,%d) exceeds declared box (%d,%d)", dw, dh, run.boxW, run.boxH)
	}
	w, h := geom.Length(img.Width()), geom.Length(img.Height())
	// Cross-multiplication check, allowing the single permitted rounding
	// (ScaleRound can be off by at most 1 in the scaled dimension).
	lhs := dw * h
	rhs := dh * w
	diff := lhs - rhs
	if diff < 0 {
		diff = -diff
	}
	tolerance := h // generous bound: one unit of rounding in dh, scaled by h
	if diff > tolerance {
		t.Fatalf("drawn box (%d,%d) does not preserve aspect ratio %d:%d (cross products %d vs %d)", dw, dh, w, h, lhs, rhs)
	}
}

// TestResolveImagePlacementOddDifferenceUsesScaleRound is RP-6's
// positive control (Finding 2, Story 1.8 review, fixing two compounding
// defects in the original version of this test):
//
//  1. The expected offsets below are LITERAL, hand-computed integers,
//     never derived from geom.ScaleRound itself — the original test
//     computed its expectation as `geom.ScaleRound(run.boxW-dw, 1, 2)`,
//     the same expression the production code under test evaluates, so
//     it could only fail if two evaluations of one expression disagreed
//     with each other. Replacing both `ScaleRound` calls with `/2` (RP-6)
//     left this test, and the whole module, green.
//
//  2. "An odd difference" is not by itself a discriminating fixture.
//     Measured directly against ScaleRound: round-half-to-even and
//     truncation agree on MOST odd values (v=5: ScaleRound=2, v/2=2;
//     v=9: ScaleRound=4, v/2=4) and differ only when the difference is
//     ≡ 3 (mod 4) (v=3: ScaleRound=2, v/2=1; v=7: ScaleRound=4, v/2=3).
//     This fixture is built around a free-axis difference of exactly 7.
//
// resolveImagePlacement always sets the BOUND axis's drawn dimension
// equal to the box exactly (drawW = bw, or drawH = bh — see the
// function's source), so that axis's centring offset is structurally
// zero and can never discriminate rounding modes; only the free axis
// can. (The original AC14 wording — "the retained fixture uses an odd
// difference on both axes" — describes a state the geometry cannot
// produce; corrected in the story's Acceptance Criteria alongside this
// fix.)
//
// Fixture: png3x2RGB (w=3, h=2) in a box where WIDTH binds exactly.
// boxW = 100002 (a multiple of 3, so drawW = 100002 with no rounding);
// drawH = ScaleRound(100002, 2, 3) = 66668 exactly (100002*2 = 200004,
// and 200004/3 = 66668 with zero remainder — no rounding tie to reason
// about here). boxH is set to drawH + 7 = 66675, so the free (Y) axis's
// difference is exactly 7: ScaleRound(7,1,2) rounds the exact half
// (3.5) to the nearest EVEN integer, 4; truncating (7/2) gives 3 — the
// two disagree, which is the whole point of this test.
func TestResolveImagePlacementOddDifferenceUsesScaleRound(t *testing.T) {
	run := imageRunSource{elementID: "e1", assetKey: "k", x: 0, y: 0, boxW: 100002, boxH: 66675}
	img, err := template.DecodeImageForRender("image/png", png3x2RGB, "k", "e1")
	if err != nil {
		t.Fatalf("DecodeImageForRender: %v", err)
	}
	drawX, drawY, dw, dh := resolveImagePlacement(run, img)

	// Sanity check on the fixture's own assumption: width must bind
	// (drawW == boxW exactly), or the hand-derived numbers below no
	// longer apply and this test would be asserting the wrong thing.
	if dw != run.boxW {
		t.Fatalf("fixture assumption broken: expected width to bind (dw == boxW == %d), got dw = %d", run.boxW, dw)
	}
	if dh != 66668 {
		t.Fatalf("fixture assumption broken: dh = %d, want 66668 (hand-computed ScaleRound(100002,2,3))", dh)
	}

	const wantOffsetX geom.Length = 0      // bound axis: structurally zero
	const wantOffsetY geom.Length = 4      // free axis: round-half-to-even(7,1,2), literal
	const truncatedOffsetY geom.Length = 3 // what "/2" would give instead — must NOT match wantOffsetY
	if wantOffsetY == truncatedOffsetY {
		t.Fatal("fixture is not discriminating: round-half-to-even and truncation must disagree here")
	}

	if drawX != run.x+wantOffsetX {
		t.Fatalf("drawX = %d, want x + %d = %d (bound axis, offset must be 0)", drawX, wantOffsetX, run.x+wantOffsetX)
	}
	if drawY != run.y+wantOffsetY {
		t.Fatalf("drawY = %d, want y + %d = %d (round-half-to-even of the free-axis difference 7, D-1.8.4 clause 3)", drawY, wantOffsetY, run.y+wantOffsetY)
	}
}

// TestRenderUnrecognisedMediaTypeErrorsOnlyWhenDrawn is AC11a's second
// and third surfaces exercised behaviourally: an unrecognised mediaType
// is not a load error (already tested in internal/template), and it
// becomes a RENDER error only when an element actually draws it — an
// orphaned asset of an unrecognised type must render CLEAN (RP-11's
// positive control).
func TestRenderUnrecognisedMediaTypeErrorsOnlyWhenDrawn(t *testing.T) {
	garbage := []byte("not any recognised format")
	key := sha256Hex(garbage)

	// Orphaned (no element references it): must load AND render clean.
	orphanDoc := `{
  "assets": {"` + key + `": {"data": ["` + base64Std(garbage) + `"], "mediaType": "image/webp"}},
  "bands": {"content": {"elements": []}, "pageFooter": {"elements": [], "height": 20}, "pageHeader": {"elements": [], "height": 20}},
  "fonts": {}, "locale": "en", "nextId": 1,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00", "version": "1.0"
}`
	tpl, err := ParseTemplate([]byte(orphanDoc))
	if err != nil {
		t.Fatalf("ParseTemplate (orphan): %v", err)
	}
	if _, err := Render(tpl, Data("{}"), nil, nil); err != nil {
		t.Fatalf("Render must succeed for an orphaned unrecognised-mediaType asset (RP-11): %v", err)
	}

	// Drawn (an element references it): must be a RENDER error, LOCATED
	// — D-1.8.1 (amended)'s binding verdict table: "Located, naming
	// element id, asset key and media type." Finding 4/11 (Story 1.8
	// review): the original version of this test asserted only
	// `err != nil`, which is how the caller hard-coding the element id
	// to "" (render.go) escaped review — the message rendered as
	// "element : asset <key>", a visible hole, and this assertion would
	// still have passed. imageTemplateJSON's drawn element carries id
	// "e1" (see its own doc/body above).
	drawnDoc := imageTemplateJSON(key, []string{base64Std(garbage)}, "image/webp", 50, 50)
	tpl2, err := ParseTemplate([]byte(drawnDoc))
	if err != nil {
		t.Fatalf("ParseTemplate (drawn): %v", err)
	}
	_, err = Render(tpl2, Data("{}"), nil, nil)
	if err == nil {
		t.Fatal("expected a render error when an element actually draws an unrecognised-mediaType asset")
	}
	if !strings.Contains(err.Error(), "e1") {
		t.Fatalf("error %q does not name the element id (D-1.8.1 amended)", err.Error())
	}
	if !strings.Contains(err.Error(), key) {
		t.Fatalf("error %q does not name the asset key (D-1.8.1 amended)", err.Error())
	}
	if !strings.Contains(err.Error(), "image/webp") {
		t.Fatalf("error %q does not name the media type (D-1.8.1 amended)", err.Error())
	}
}

func jpeg4x4Bytes() []byte {
	return jpeg4x4TestBytes
}

var jpeg4x4TestBytes = []byte{
	0xff, 0xd8, 0xff, 0xdb, 0x00, 0x84, 0x00, 0x03, 0x02, 0x02, 0x03, 0x02, 0x02, 0x03, 0x03, 0x03,
	0x03, 0x04, 0x03, 0x03, 0x04, 0x05, 0x08, 0x05, 0x05, 0x04, 0x04, 0x05, 0x0a, 0x07, 0x07, 0x06,
	0x08, 0x0c, 0x0a, 0x0c, 0x0c, 0x0b, 0x0a, 0x0b, 0x0b, 0x0d, 0x0e, 0x12, 0x10, 0x0d, 0x0e, 0x11,
	0x0e, 0x0b, 0x0b, 0x10, 0x16, 0x10, 0x11, 0x13, 0x14, 0x15, 0x15, 0x15, 0x0c, 0x0f, 0x17, 0x18,
	0x16, 0x14, 0x18, 0x12, 0x14, 0x15, 0x14, 0x01, 0x03, 0x04, 0x04, 0x05, 0x04, 0x05, 0x09, 0x05,
	0x05, 0x09, 0x14, 0x0d, 0x0b, 0x0d, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14,
	0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14,
	0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14,
	0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0x14, 0xff, 0xc0, 0x00, 0x11, 0x08, 0x00, 0x04, 0x00,
	0x04, 0x03, 0x01, 0x22, 0x00, 0x02, 0x11, 0x01, 0x03, 0x11, 0x01, 0xff, 0xc4, 0x01, 0xa2, 0x00,
	0x00, 0x01, 0x05, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x10, 0x00, 0x02, 0x01,
	0x03, 0x03, 0x02, 0x04, 0x03, 0x05, 0x05, 0x04, 0x04, 0x00, 0x00, 0x01, 0x7d, 0x01, 0x02, 0x03,
	0x00, 0x04, 0x11, 0x05, 0x12, 0x21, 0x31, 0x41, 0x06, 0x13, 0x51, 0x61, 0x07, 0x22, 0x71, 0x14,
	0x32, 0x81, 0x91, 0xa1, 0x08, 0x23, 0x42, 0xb1, 0xc1, 0x15, 0x52, 0xd1, 0xf0, 0x24, 0x33, 0x62,
	0x72, 0x82, 0x09, 0x0a, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x25, 0x26, 0x27, 0x28, 0x29, 0x2a, 0x34,
	0x35, 0x36, 0x37, 0x38, 0x39, 0x3a, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49, 0x4a, 0x53, 0x54,
	0x55, 0x56, 0x57, 0x58, 0x59, 0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6a, 0x73, 0x74,
	0x75, 0x76, 0x77, 0x78, 0x79, 0x7a, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89, 0x8a, 0x92, 0x93,
	0x94, 0x95, 0x96, 0x97, 0x98, 0x99, 0x9a, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7, 0xa8, 0xa9, 0xaa,
	0xb2, 0xb3, 0xb4, 0xb5, 0xb6, 0xb7, 0xb8, 0xb9, 0xba, 0xc2, 0xc3, 0xc4, 0xc5, 0xc6, 0xc7, 0xc8,
	0xc9, 0xca, 0xd2, 0xd3, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda, 0xe1, 0xe2, 0xe3, 0xe4, 0xe5,
	0xe6, 0xe7, 0xe8, 0xe9, 0xea, 0xf1, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8, 0xf9, 0xfa, 0x01,
	0x00, 0x03, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x11, 0x00, 0x02, 0x01,
	0x02, 0x04, 0x04, 0x03, 0x04, 0x07, 0x05, 0x04, 0x04, 0x00, 0x01, 0x02, 0x77, 0x00, 0x01, 0x02,
	0x03, 0x11, 0x04, 0x05, 0x21, 0x31, 0x06, 0x12, 0x41, 0x51, 0x07, 0x61, 0x71, 0x13, 0x22, 0x32,
	0x81, 0x08, 0x14, 0x42, 0x91, 0xa1, 0xb1, 0xc1, 0x09, 0x23, 0x33, 0x52, 0xf0, 0x15, 0x62, 0x72,
	0xd1, 0x0a, 0x16, 0x24, 0x34, 0xe1, 0x25, 0xf1, 0x17, 0x18, 0x19, 0x1a, 0x26, 0x27, 0x28, 0x29,
	0x2a, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3a, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49, 0x4a, 0x53,
	0x54, 0x55, 0x56, 0x57, 0x58, 0x59, 0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6a, 0x73,
	0x74, 0x75, 0x76, 0x77, 0x78, 0x79, 0x7a, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89, 0x8a,
	0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98, 0x99, 0x9a, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7, 0xa8,
	0xa9, 0xaa, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6, 0xb7, 0xb8, 0xb9, 0xba, 0xc2, 0xc3, 0xc4, 0xc5, 0xc6,
	0xc7, 0xc8, 0xc9, 0xca, 0xd2, 0xd3, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda, 0xe2, 0xe3, 0xe4,
	0xe5, 0xe6, 0xe7, 0xe8, 0xe9, 0xea, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8, 0xf9, 0xfa, 0xff,
	0xda, 0x00, 0x0c, 0x03, 0x01, 0x00, 0x02, 0x11, 0x03, 0x11, 0x00, 0x3f, 0x00, 0xb1, 0xf0, 0xf7,
	0xf6, 0x7e, 0xf0, 0x57, 0xfc, 0x22, 0xf6, 0xbf, 0xf1, 0x2c, 0xfd, 0x47, 0xa0, 0xf6, 0xae, 0x93,
	0xfe, 0x19, 0xfb, 0xc1, 0x5f, 0xf4, 0x0c, 0xfd, 0x47, 0xf8, 0x57, 0x47, 0xf0, 0xf7, 0xfe, 0x45,
	0x7b, 0x5f, 0xf3, 0xd8, 0x57, 0x49, 0x5f, 0x99, 0x62, 0xb3, 0x4c, 0x77, 0xb7, 0x9f, 0xef, 0xa5,
	0xbb, 0xea, 0xce, 0xec, 0x87, 0x3a, 0xcc, 0xbf, 0xb2, 0xb0, 0xdf, 0xed, 0x13, 0xf8, 0x23, 0xf6,
	0x9f, 0x63, 0xff, 0xd9,
}

// TestRenderReadsAssetBytesFromTheDocumentItself is AC20 (D-1.8.5):
// mutate the in-memory asset bytes and assert the rendered PDF changes.
// If the renderer read from disk, a cache, or anywhere but the
// document, it would not — this proves the provenance claim rather than
// restating it (the assertion is on the rendered bytes DIFFERING, not
// on a field having been read).
func TestRenderReadsAssetBytesFromTheDocumentItself(t *testing.T) {
	key := sha256Hex(png3x2RGB)
	doc := imageTemplateJSON(key, wrap76(png3x2RGB), "image/png", 100, 100)
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	out1Res, err := Render(tpl, Data("{}"), nil, nil)
	if err != nil {
		t.Fatalf("Render (first): %v", err)
	}
	out1 := out1Res.Bytes

	// Mutate the in-memory asset's bytes directly on the parsed
	// Document — a different, still-valid image with the SAME box, so
	// only the embedded stream bytes differ.
	asset := tpl.doc.Assets[key]
	// Swap in different (still valid, still supported) PNG bytes under
	// the SAME map key and element reference, re-wrapped canonically.
	otherImage := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x00, 0x00, 0x00, 0x00, 0x3a, 0x7e, 0x9b,
		0x55, 0x00, 0x00, 0x00, 0x0e, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x62, 0x6a, 0x00, 0x04, 0x00,
		0x00, 0xff, 0xff, 0x00, 0x86, 0x00, 0x83, 0x4c, 0x9e, 0x20, 0x4b, 0x00, 0x00, 0x00, 0x00, 0x49,
		0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
	asset.Data = wrap76(otherImage)
	tpl.doc.Assets[key] = asset

	out2Res, err := Render(tpl, Data("{}"), nil, nil)
	if err != nil {
		t.Fatalf("Render (mutated): %v", err)
	}
	out2 := out2Res.Bytes

	if bytes.Equal(out1, out2) {
		t.Fatal("AC20: mutating the in-memory asset bytes did not change the rendered output — the renderer may not be reading from the document itself")
	}
}
