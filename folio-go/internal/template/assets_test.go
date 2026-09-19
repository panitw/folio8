package template

import (
	"encoding/base64"
	"strings"
	"testing"
)

// assetDoc builds a minimal, otherwise-valid document JSON carrying one
// asset entry with the given key/data/mediaType, and — if referencedBy
// is non-empty — one image element referencing it (used to exercise
// AC8's orphan case when referencedBy is "").
func assetDoc(key, dataJSON, mediaType, referencedBy string) []byte {
	var elements string
	if referencedBy != "" {
		elements = `{"id":"e1","type":"image","asset":"` + referencedBy + `","x":0,"y":0,"width":50,"height":50}`
	}
	doc := `{
  "assets": {"` + key + `": {"data": ` + dataJSON + `, "mediaType": "` + mediaType + `"}},
  "bands": {"content": {"elements": [` + elements + `]}, "pageFooter": {"elements": [], "height": 20}, "pageHeader": {"elements": [], "height": 20}},
  "fonts": {},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}`
	return []byte(doc)
}

func dataArrayJSON(b []byte) string {
	parts := splitBase64Canonical(b)
	quoted := make([]string, len(parts))
	for i, p := range parts {
		quoted[i] = `"` + p + `"`
	}
	return "[" + strings.Join(quoted, ",") + "]"
}

// TestParseDocumentRejectsMalformedAssetKeyShape is RP-4a: a 62-character
// key must be rejected with a SHAPE message, distinguishable from a
// well-formed-but-wrong VALUE (RP-4).
func TestParseDocumentRejectsMalformedAssetKeyShape(t *testing.T) {
	key := strings.Repeat("c", 62)
	b := assetDoc(key, dataArrayJSON(png3x2RGB), "image/png", "")
	_, err := ParseDocument(b)
	if err == nil {
		t.Fatal("expected an error for a 62-character asset key")
	}
	if !strings.Contains(err.Error(), "64-character lowercase hex digest") {
		t.Fatalf("expected the SHAPE message, got: %v", err)
	}
}

// TestParseDocumentRejectsWrongAssetKeyValue is RP-4: a well-formed
// 64-hex key that does not match the SHA-256 of its data must be
// rejected with a VALUE message, distinguishable from the shape error
// above.
func TestParseDocumentRejectsWrongAssetKeyValue(t *testing.T) {
	wrongKey := strings.Repeat("a", 64) // well-formed shape, wrong value
	b := assetDoc(wrongKey, dataArrayJSON(png3x2RGB), "image/png", "")
	_, err := ParseDocument(b)
	if err == nil {
		t.Fatal("expected an error for a mismatched asset key")
	}
	if !strings.Contains(err.Error(), "does not match the SHA-256 of its data") {
		t.Fatalf("expected the VALUE message, got: %v", err)
	}
	if strings.Contains(err.Error(), "64-character lowercase hex digest") {
		t.Fatalf("a well-formed-but-wrong key must NOT reuse the shape message: %v", err)
	}
}

// TestParseDocumentAcceptsCorrectAssetKey is the positive control for
// both tests above.
func TestParseDocumentAcceptsCorrectAssetKey(t *testing.T) {
	key := sha256HexOf(png3x2RGB)
	b := assetDoc(key, dataArrayJSON(png3x2RGB), "image/png", "")
	if _, err := ParseDocument(b); err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
}

// TestParseDocumentRejectsEmptyDecodedAsset is AC4: an asset whose
// decoded data is empty is a load error.
func TestParseDocumentRejectsEmptyDecodedAsset(t *testing.T) {
	key := sha256HexOf([]byte{})
	b := assetDoc(key, `[""]`, "image/png", "")
	_, err := ParseDocument(b)
	if err == nil {
		t.Fatal("expected an error for an empty decoded asset")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected an 'empty' diagnosis, got: %v", err)
	}
}

// TestParseDocumentPreservesOrphanedAsset is AC8 (RP-3's positive):
// an asset referenced by no element still loads and round-trips —
// D-1.4.3's P1 forces preservation, there is no policy latitude to drop
// it.
func TestParseDocumentPreservesOrphanedAsset(t *testing.T) {
	key := sha256HexOf(png3x2RGB)
	b := assetDoc(key, dataArrayJSON(png3x2RGB), "image/png", "")
	d, err := ParseDocument(b)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, ok := d.Assets[key]; !ok {
		t.Fatalf("orphaned asset %q was dropped on parse", key)
	}
	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatalf("SerializeDocument: %v", err)
	}
	if !strings.Contains(string(out), key) {
		t.Fatal("orphaned asset was dropped by the serializer — AC8/P1 violated")
	}
}

// TestParseDocumentUnrecognisedMediaTypeNeverALoadError is AC11a's
// first surface (D-1.8.1 amended): an asset of an unrecognised
// mediaType loads cleanly regardless of its bytes — never inspected,
// never refused, at load time.
func TestParseDocumentUnrecognisedMediaTypeNeverALoadError(t *testing.T) {
	garbage := []byte("this is not any recognised image format at all")
	key := sha256HexOf(garbage)
	b := assetDoc(key, dataArrayJSON(garbage), "image/webp", "")
	if _, err := ParseDocument(b); err != nil {
		t.Fatalf("an unrecognised mediaType must never be a load error, got: %v", err)
	}
}

// TestParseDocumentRecognisedButUnparseableIsLoadError is AC11b's
// contrast case (RP-12's positive control): a RECOGNISED mediaType
// (image/png) whose bytes are not that format is a load error — the
// file lies about itself, reader-independent.
func TestParseDocumentRecognisedButUnparseableIsLoadError(t *testing.T) {
	notAPNG := []byte("this is not a PNG despite the label")
	key := sha256HexOf(notAPNG)
	b := assetDoc(key, dataArrayJSON(notAPNG), "image/png", "")
	if _, err := ParseDocument(b); err == nil {
		t.Fatal("expected a load error for image/png bytes that are not a PNG")
	}
}

// TestParseDocumentAlphaPNGIsLoadError is AC9a: an alpha PNG arrives
// under a RECOGNISED mediaType, so it is parsed and rejected on its
// content at LOAD time — not deferred to render.
func TestParseDocumentAlphaPNGIsLoadError(t *testing.T) {
	key := sha256HexOf(png2x2Alpha)
	b := assetDoc(key, dataArrayJSON(png2x2Alpha), "image/png", "")
	_, err := ParseDocument(b)
	if err == nil {
		t.Fatal("expected a load error for an alpha PNG")
	}
	if !strings.Contains(err.Error(), "alpha") {
		t.Fatalf("expected the error to name alpha explicitly, got: %v", err)
	}
}

func init() {
	// Sanity: dataArrayJSON must actually produce valid canonical JSON
	// string array syntax (guards against a future edit to
	// splitBase64Canonical breaking this test file's own fixture
	// builder silently).
	if !strings.HasPrefix(dataArrayJSON([]byte("x")), "[") {
		panic("dataArrayJSON: malformed output")
	}
	_ = base64.StdEncoding
}
