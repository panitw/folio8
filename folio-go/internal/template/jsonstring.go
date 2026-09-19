package template

// This file is the canonical serializer's string- and whitespace-emission
// core (AD-9). Every string anywhere in a serialized `.folio` document —
// known field or passthrough — goes through appendJSONString, so the
// three escaping traps (AC19 HTML off, AC20 literal UTF-8, AC21 minimal
// escaping) are enforced in exactly one place.

// appendIndent appends depth*2 spaces (AC18: two-space indentation).
func appendIndent(dst []byte, depth int) []byte {
	for i := 0; i < depth; i++ {
		dst = append(dst, ' ', ' ')
	}
	return dst
}

// appendJSONString appends the canonical, minimally-escaped, quoted JSON
// text form of s to dst.
//
// AC19 — HTML escaping is OFF: '<', '>' and '&' are emitted literally,
// never as < / > / & (encoding/json's SetEscapeHTML(false)
// behaviour, reimplemented by hand here since this function does not use
// encoding/json at all).
//
// AC20 — UTF-8 is emitted literally: no \uXXXX escape is ever produced
// for a rune above 0x1F. s is a Go string (UTF-8 by construction) and is
// copied through byte-for-byte outside the escaped set.
//
// AC21 — minimal escaping only: '"', '\\', and control characters below
// 0x20 are escaped (using the short forms \b \f \n \r \t where they
// apply, \u00XX otherwise); '/' is never escaped.
func appendJSONString(dst []byte, s string) []byte {
	dst = append(dst, '"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			dst = append(dst, '\\', '"')
		case c == '\\':
			dst = append(dst, '\\', '\\')
		case c == '\b':
			dst = append(dst, '\\', 'b')
		case c == '\f':
			dst = append(dst, '\\', 'f')
		case c == '\n':
			dst = append(dst, '\\', 'n')
		case c == '\r':
			dst = append(dst, '\\', 'r')
		case c == '\t':
			dst = append(dst, '\\', 't')
		case c < 0x20:
			dst = append(dst, '\\', 'u', '0', '0', hexDigit(c>>4), hexDigit(c&0xF))
		default:
			// Includes '<', '>', '&', '/' (never escaped) and every
			// UTF-8 continuation/lead byte of a multi-byte rune, copied
			// through literally (AC20).
			dst = append(dst, c)
		}
	}
	dst = append(dst, '"')
	return dst
}

func hexDigit(n byte) byte {
	if n < 10 {
		return '0' + n
	}
	return 'a' + (n - 10)
}
