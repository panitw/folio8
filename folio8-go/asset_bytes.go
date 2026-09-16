package folio8

import (
	"fmt"

	"github.com/panitw/folio8/folio8-go/internal/template"
)

// assetBytes returns one asset's raw, decoded bytes and declared media type
// by its content-addressed key. It is Story 5.13's per-key paintable-bytes
// route (D-5.13.2's "Producer" clause): the canvas projection itself
// (CanvasImagePaint) carries geometry and identity only, never bytes
// (AD-17), so this is the browser's ONLY way to obtain the bytes it paints
// with — one explicit request per asset key, mirroring the existing
// EngineSuccess.bytes channel Serialize already uses outside the snapshot.
//
// It is a read-only query: it never touches revision, undo/redo history, or
// canonical bytes.
func assetBytes(t *Template, key string) ([]byte, string, error) {
	if t == nil {
		return nil, "", errNilTemplate
	}
	if !isAssetKeyShape(key) {
		return nil, "", fmt.Errorf("folio8: asset key is not a 64-character lowercase hex digest")
	}
	asset, ok := t.doc.Assets[key]
	if !ok {
		return nil, "", fmt.Errorf("folio8: asset %q is not present in the document", key)
	}
	raw, err := template.DecodeAssetBytes(asset)
	if err != nil {
		return nil, "", err
	}
	return raw, asset.MediaType, nil
}

// isAssetKeyShape mirrors internal/template's own key-shape check
// (isSHA256HexKey, assetkey.go): 64 lowercase hex characters. Duplicated
// rather than exported across the package boundary, matching that file's
// own unexported-and-package-local precedent.
func isAssetKeyShape(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
