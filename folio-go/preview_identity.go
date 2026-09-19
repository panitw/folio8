package folio8

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"maps"
	"slices"
)

// previewIdentity returns opaque evidence for the exact five inputs that can
// affect a production preview. It is deliberately an engine-side byte
// protocol: callers compare the returned digest, but never recreate it.
func previewIdentity(template []byte, data Data, params Params, fontSet FontSet) string {
	h := sha256.New()
	writePreviewField(h, "folio8-preview-identity/v1", nil)
	writePreviewField(h, "template", template)
	writePreviewField(h, "data", data)
	writePreviewField(h, "params", params)
	writePreviewField(h, "folio8-version", []byte(Version))

	names := slices.Sorted(maps.Keys(fontSet))
	for _, name := range names {
		writePreviewField(h, "font-face", []byte(name))
		writePreviewField(h, "font-program", fontSet[name])
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}

// writePreviewField is ordered, labelled and length-delimited so adjacent
// values cannot be confused through concatenation boundaries.
func writePreviewField(h interface{ Write([]byte) (int, error) }, label string, value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(label)))
	_, _ = h.Write(length[:])
	_, _ = h.Write([]byte(label))
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = h.Write(length[:])
	_, _ = h.Write(value)
}
