package template

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// rawIsNull reports whether raw's trimmed bytes are the literal JSON
// null (used to distinguish absent-from-object vs explicit null,
// D-1.4.8).
func rawIsNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

// decodeObjectMap decodes raw as a JSON object into a
// map[string]json.RawMessage, erroring if raw is not a JSON object.
// Values stay as undecoded RawMessage — UseNumber is irrelevant at
// this level, since no number is decoded yet.
func decodeObjectMap(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("template: expected a JSON object: %w", err)
	}
	if m == nil {
		m = map[string]json.RawMessage{}
	}
	return m, nil
}

// decodeArrayRaw decodes raw as a JSON array into a slice of undecoded
// RawMessage elements, preserving order (array element order is
// authored content, never sorted).
func decodeArrayRaw(raw json.RawMessage) ([]json.RawMessage, error) {
	var a []json.RawMessage
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, fmt.Errorf("template: expected a JSON array: %w", err)
	}
	if a == nil {
		a = []json.RawMessage{}
	}
	return a, nil
}

// decodeStringRaw decodes raw as a JSON string. Never coerces (AC40):
// a number or bool where a string belongs is an error, not a
// stringified conversion.
func decodeStringRaw(raw json.RawMessage) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", fmt.Errorf("template: expected a JSON string: %w", err)
	}
	return s, nil
}

// decodeBoolRaw decodes raw as a JSON bool. Never coerces (AC40).
func decodeBoolRaw(raw json.RawMessage) (bool, error) {
	var b bool
	if err := json.Unmarshal(raw, &b); err != nil {
		return false, fmt.Errorf("template: expected a JSON bool: %w", err)
	}
	return b, nil
}

// decodeNumberRaw decodes raw as a JSON number literal, preserved
// exactly (json.Number's UnmarshalJSON accepts the target type
// explicitly, so this works correctly without a decoder-level
// UseNumber() call — UseNumber only changes interface{} decoding,
// AC26). Never coerces: a JSON string where a number belongs is
// rejected here, not parsed (AC40).
func decodeNumberRaw(raw json.RawMessage) (json.Number, error) {
	trimmed := bytes.TrimSpace(raw)
	// json.Number is, under the hood, `type Number string` — and
	// encoding/json's default string decoding happily accepts a quoted
	// JSON string into it with NO error (measured: json.Unmarshal(`"0"`,
	// &n) succeeds silently). That is exactly the coercion AC40 forbids
	// ("never a parse-and-convert"), so the token's own first byte is
	// checked explicitly: a JSON number is never a quoted string, so a
	// leading '"' is rejected here before json.Number ever sees it.
	if len(trimmed) > 0 && trimmed[0] == '"' {
		return "", fmt.Errorf("template: expected a JSON number, got a JSON string %s — never coerced", trimmed)
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err != nil {
		return "", fmt.Errorf("template: expected a JSON number: %w", err)
	}
	return n, nil
}

// decodeStringArrayRaw decodes raw as a JSON array of strings,
// preserving order.
func decodeStringArrayRaw(raw json.RawMessage) ([]string, error) {
	items, err := decodeArrayRaw(raw)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		s, err := decodeStringRaw(it)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// extraFields builds the sorted passthrough Field list for every key in
// obj not present in consumed (AC8, AC9, AC18: sorted by byte-order —
// this reuses decodeRaw's own sort, since rawFromAny already sorts
// nested objects; the top-level set of unconsumed keys here is sorted
// explicitly).
func extraFields(obj map[string]json.RawMessage, consumed map[string]bool) ([]Field, error) {
	var keys []string
	// D-1.3.5/NFR1.d: ranging a map is forbidden anywhere under
	// internal/ (iteration order is unspecified and so non-deterministic
	// across runs) — sorted keys first, then index the map by key.
	for _, k := range slices.Sorted(maps.Keys(obj)) {
		if !consumed[k] {
			keys = append(keys, k)
		}
	}
	fields := make([]Field, 0, len(keys))
	for _, k := range keys {
		v, err := decodeRaw(obj[k])
		if err != nil {
			return nil, fmt.Errorf("template: unknown key %q: %w", k, err)
		}
		fields = append(fields, Field{Key: k, Value: v})
	}
	return fields, nil
}

// decodeUnbreakableValues decodes the document-level `unbreakableValues`
// list (Story 2.4; D-2.1.6 OWNER, D-2.4.1) and validates every entry
// against D-1.4.1's path convention — "a bare root-relative dotted value
// path… No `{{ }}`, no function call, no `[]`", the same convention
// `footerOf` already uses.
//
// VALIDATING IT HERE IS WHAT MAKES THE CONVENTION REAL. A key that
// accepted "{{customer.name}}" or "sum(x)" would quietly grow a second
// path dialect, and the entry would then silently match nothing at
// render time — a declaration that appears honoured and is not. That is
// worse than a load error, because it fails in the direction of
// splitting a customer's name with no diagnostic anywhere.
//
// The grammar is Story 1.6's, deliberately restated rather than
// imported: internal/template must not import internal/bind (the two
// packages' numeric-spelling functions are already deliberately
// unshared for the same reason, AC25), and a parse-time schema check is
// not a bind-time resolution.
func decodeUnbreakableValues(raw json.RawMessage) ([]string, error) {
	paths, err := decodeStringArrayRaw(raw)
	if err != nil {
		return nil, newLoadError("unbreakableValues", "", string(raw), "must be an array of strings: "+err.Error())
	}
	seen := make(map[string]bool, len(paths))
	for _, p := range paths {
		if !isBareDottedPath(p) {
			return nil, newLoadError("unbreakableValues", "", p,
				"must be a bare root-relative dotted data path such as \"customer.name\" (D-1.4.1, the same convention footerOf uses): "+
					"no \"{{ }}\", no function call, no \"[]\", no whitespace")
		}
		if seen[p] {
			return nil, newLoadError("unbreakableValues", "", p, "listed more than once — a duplicate declaration is never repaired and never silently collapsed")
		}
		seen[p] = true
	}
	return paths, nil
}

// isBareDottedPath reports whether p matches D-1.4.1's path convention:
//
//	path    := ident ( "." ident )*
//	ident   := [A-Za-z_][A-Za-z0-9_]*
//
// Identical to the grammar internal/bind accepts inside "{{ }}"
// (D-1.6.5), minus the braces — which is the point of D-2.4.1's "one
// path convention in the format, not two".
func isBareDottedPath(p string) bool {
	if p == "" {
		return false
	}
	for _, seg := range strings.Split(p, ".") {
		if seg == "" {
			return false
		}
		for i := 0; i < len(seg); i++ {
			c := seg[i]
			switch {
			case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c == '_':
			case c >= '0' && c <= '9':
				if i == 0 {
					return false
				}
			default:
				return false
			}
		}
	}
	return true
}
