package template

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
)

// RawKind is the discriminant of a RawValue.
type RawKind int

const (
	RawNull RawKind = iota
	RawString
	RawNumber
	RawBool
	RawArray
	RawObject
)

// RawValue is a fully generic, canonicalised JSON value: the shape
// passthrough content takes at every nesting level (AC8, AC9). An
// object's Obj is always sorted by byte-order key (AC18) — sorting
// happens once, at decode time, so the in-memory Document is always
// already in its canonical shape and the serializer never has to sort
// passthrough content separately from known fields.
//
// Numbers are preserved as their exact original literal text (Num,
// via json.Number) rather than renormalised: folio-format.md defines a
// canonical spelling only for the two KNOWN numeric kinds (points and
// nextId, D-1.4.3); an opaque passthrough number has no such rule, so
// "canonical" for it is simply "written back exactly as read" — which
// still satisfies P1/P2/P3 for the passthrough fixture (AC9), since
// re-parsing an unchanged literal reproduces the same RawValue.
type RawValue struct {
	Kind RawKind
	Str  string
	Num  string
	Bool bool
	Arr  []RawValue
	Obj  []Field
}

// decodeRaw parses msg (already known to be syntactically valid JSON,
// since it came from a json.RawMessage produced by a prior decode pass)
// into a canonicalised RawValue, sorting every object's keys by
// byte-order as it goes.
func decodeRaw(msg json.RawMessage) (RawValue, error) {
	dec := json.NewDecoder(bytes.NewReader(msg))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return RawValue{}, err
	}
	return rawFromAny(v), nil
}

func rawFromAny(v interface{}) RawValue {
	switch x := v.(type) {
	case nil:
		return RawValue{Kind: RawNull}
	case bool:
		return RawValue{Kind: RawBool, Bool: x}
	case json.Number:
		return RawValue{Kind: RawNumber, Num: string(x)}
	case string:
		return RawValue{Kind: RawString, Str: x}
	case []interface{}:
		arr := make([]RawValue, 0, len(x))
		for _, e := range x {
			arr = append(arr, rawFromAny(e))
		}
		return RawValue{Kind: RawArray, Arr: arr}
	case map[string]interface{}:
		// D-1.3.5/NFR1.d bans ranging a map anywhere under internal/;
		// slices.Sorted(maps.Keys(x)) both satisfies that and gives
		// AC18's byte-order key sort in the same step (Go's < on string
		// is byte-order over UTF-8).
		obj := make([]Field, 0, len(x))
		for _, k := range slices.Sorted(maps.Keys(x)) {
			obj = append(obj, Field{Key: k, Value: rawFromAny(x[k])})
		}
		return RawValue{Kind: RawObject, Obj: obj}
	default:
		panic(fmt.Sprintf("template: decodeRaw: unexpected decoded type %T", v))
	}
}

// appendRaw appends v's canonical bytes to dst at the given indent
// depth (AC18: two-space indent per level), honouring the same
// escaping rules as every other string in the document (AC19–AC21).
func appendRaw(dst []byte, v RawValue, depth int) []byte {
	switch v.Kind {
	case RawNull:
		return append(dst, "null"...)
	case RawBool:
		if v.Bool {
			return append(dst, "true"...)
		}
		return append(dst, "false"...)
	case RawNumber:
		return append(dst, v.Num...)
	case RawString:
		return appendJSONString(dst, v.Str)
	case RawArray:
		return appendRawArray(dst, v.Arr, depth)
	case RawObject:
		return appendRawObject(dst, v.Obj, depth)
	default:
		panic("template: appendRaw: unknown RawKind")
	}
}

func appendRawArray(dst []byte, arr []RawValue, depth int) []byte {
	if len(arr) == 0 {
		return append(dst, "[]"...)
	}
	dst = append(dst, '[')
	for i, e := range arr {
		if i > 0 {
			dst = append(dst, ',')
		}
		dst = append(dst, '\n')
		dst = appendIndent(dst, depth+1)
		dst = appendRaw(dst, e, depth+1)
	}
	dst = append(dst, '\n')
	dst = appendIndent(dst, depth)
	dst = append(dst, ']')
	return dst
}

func appendRawObject(dst []byte, obj []Field, depth int) []byte {
	if len(obj) == 0 {
		return append(dst, "{}"...)
	}
	dst = append(dst, '{')
	for i, f := range obj {
		if i > 0 {
			dst = append(dst, ',')
		}
		dst = append(dst, '\n')
		dst = appendIndent(dst, depth+1)
		dst = appendJSONString(dst, f.Key)
		dst = append(dst, ':', ' ')
		dst = appendRaw(dst, f.Value, depth+1)
	}
	dst = append(dst, '\n')
	dst = appendIndent(dst, depth)
	dst = append(dst, '}')
	return dst
}
