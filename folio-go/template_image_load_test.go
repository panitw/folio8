package folio8

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

// truncatedInHeaderPNG is the PNG signature (8 bytes) plus one chunk
// header (4-byte length + 4-byte type, IHDR, declared length 13) — 16
// bytes total, with NO chunk body and NO CRC. This is Finding 1's
// (Story 1.8 review) exact reproduction shape: it lands in decodePNG's
// length bounds check with zero bytes remaining after the chunk header
// is consumed — the window the pre-fix `uint32(len(data)-pos)-4`
// comparison wrapped to ~4.29e9 on, letting the broken guard pass and
// the next line slice past the end of the buffer. It is asserted here
// through the PUBLIC ParseTemplate entry point — the library's
// untrusted-input boundary — not merely decodePNG's own unit test.
var truncatedInHeaderPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
}

// TestParseTemplateRejectsPNGTruncatedInHeaderWithoutPanicking is the
// public-API reproduction of Finding 1 (Story 1.8 review): a malformed
// asset must reach the caller as a located, named *template.LoadError,
// never as a runtime panic — the exact promise AC15/AC15b and D-1.8.4
// clause 4 make and D-1.5.2 exists to guard end to end.
func TestParseTemplateRejectsPNGTruncatedInHeaderWithoutPanicking(t *testing.T) {
	sum := sha256.Sum256(truncatedInHeaderPNG)
	key := hex.EncodeToString(sum[:])
	data := base64.StdEncoding.EncodeToString(truncatedInHeaderPNG)

	doc := fmt.Sprintf(`{
  "assets": {
    %q: {
      "data": [%q],
      "mediaType": "image/png"
    }
  },
  "bands": {
    "content": { "elements": [] },
    "pageFooter": { "elements": [], "height": 20 },
    "pageHeader": { "elements": [], "height": 20 }
  },
  "fonts": {},
  "locale": "en",
  "nextId": 1,
  "page": {
    "margin": { "bottom": 36, "left": 36, "right": 36, "top": 36 },
    "orientation": "portrait",
    "size": "A4"
  },
  "utcOffset": "+00:00",
  "version": "1.0"
}
`, key, data)

	// This must not panic. If it does, the test binary crashes and the
	// failure is reported by the test runner rather than a normal
	// t.Fatal — recorded here explicitly per Finding 11 (D-000.13):
	// asserting only "no panic" by virtue of reaching the line below
	// would be silent about *why* it passed, so the message is checked
	// too.
	_, err := ParseTemplate([]byte(doc))
	if err == nil {
		t.Fatal("expected a located load error for a PNG truncated inside a chunk header, got nil")
	}
	if !strings.Contains(err.Error(), "truncated PNG") {
		t.Fatalf("error %q does not name the truncation (D-000.13)", err.Error())
	}
	if !strings.Contains(err.Error(), key) {
		t.Fatalf("error %q is not located — does not name the asset key", err.Error())
	}
}
