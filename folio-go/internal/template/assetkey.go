package template

import (
	"crypto/sha256"
	"encoding/hex"
)

// isSHA256HexKey reports whether s is exactly 64 lowercase hex
// characters — AC6a's SHAPE check (D-1.8.8), run before the VALUE check
// (sha256HexOf comparison) because a key that fails shape is evidence
// nothing looked at the key at all, a different diagnosis from a
// well-formed key that does not match its data, and the shape check is
// the cheaper one.
func isSHA256HexKey(s string) bool {
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

// sha256HexOf returns the lowercase hex SHA-256 digest of b — AC5's key
// definition, applied to the DECODED bytes (never the base64 text).
func sha256HexOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
