package bind

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"

	"github.com/panitw/folio8/folio-go/internal/expr"
)

// Kind is the discriminant of a Value.
type Kind int

const (
	KindNull Kind = iota
	KindString
	KindNumber
	KindBool
	KindArray
	KindObject
)

func (k Kind) String() string {
	switch k {
	case KindNull:
		return "null"
	case KindString:
		return "string"
	case KindNumber:
		return "number"
	case KindBool:
		return "bool"
	case KindArray:
		return "array"
	case KindObject:
		return "object"
	default:
		return "unknown"
	}
}

// Value is bind's generic report-data value tree (AC7, AC24) — the
// SECOND of this story's two decode trees. internal/template's Document
// (Story 1.4) is schema-typed, built from the `.folio` document itself;
// Value is generic, built from whatever JSON shape a caller's report
// data happens to be. The two tree-builders stay deliberately separate
// (AC7: merging them would force the `.folio` schema to model arbitrary
// caller JSON) — what they SHARE is only the number-literal
// decomposition (internal/template.SplitJSONNumber, via Decimal, AC6).
type Value struct {
	Kind Kind
	Str  string
	Num  string // the literal's own text, exact (json.Number, via UseNumber)
	Bool bool
	Arr  []Value
	Obj  map[string]Value
}

// AsDecimal converts a KindNumber Value's literal into an
// internal/expr.Decimal (AC1, AC6), reusing expr.NewDecimal and,
// through it, the module's one splitter. Decimal itself moved to
// internal/expr at Story 3.2 (D-3.2.1, DW-8): bind (stage rank 4)
// imports expr (rank 3) to reuse it, never the reverse.
func (v Value) AsDecimal() (expr.Decimal, error) {
	if v.Kind != KindNumber {
		return expr.Decimal{}, fmt.Errorf("bind: value is a %s, not a number", v.Kind)
	}
	return expr.NewDecimal(v.Num)
}

// Presence is the outcome of Lookup — AD-14's three cases (D-1.6.2,
// AC8-AC11): a path can be absent, explicitly null, or present with a
// value. A bare pointer/bool cannot represent this distinction (the
// same reason internal/template's Presence[T] exists, D-1.4.8): here
// it is an enum rather than a generic wrapper because Lookup already
// walks a Value tree of its own Kind, not a struct field.
type Presence int

const (
	// Absent: no key of that name exists at that point in the tree
	// (AC8).
	Absent Presence = iota
	// Null: the key exists and its value is JSON null (AC9).
	Null
	// Present: the key exists with a non-null value (AC10 tests its
	// Kind at the binding site).
	Present
)

// Lookup resolves a dotted path (AC15's grammar, already split into
// segments) against v, walking nested objects only — arrays are not
// part of this story's grammar (AC15). A path that runs through a
// non-object at any but the last segment, or whose final segment's key
// does not exist in its parent object, is Absent (AC8): "the report
// does not contain this" covers both "no such key" and "the path does
// not even make sense against this shape" identically, since neither
// is information report data can be expected to supply.
func (v Value) Lookup(path []string) (Value, Presence) {
	cur := v
	for i, seg := range path {
		if cur.Kind != KindObject {
			return Value{}, Absent
		}
		child, ok := cur.Obj[seg]
		if !ok {
			return Value{}, Absent
		}
		if i == len(path)-1 {
			if child.Kind == KindNull {
				return child, Null
			}
			return child, Present
		}
		cur = child
	}
	// path is never empty at a real call site (internal/expr's parser
	// rejects an empty binding, and exprResolver.Resolve — text.go —
	// has its own len(path)==0 guard; parseBindingPath, the pre-3.2
	// matcher this comment used to cite, was deleted by AC23), but an
	// empty path resolves to v itself for completeness rather than
	// panicking.
	if v.Kind == KindNull {
		return v, Null
	}
	return v, Present
}

// DecodeData parses d (a caller's JSON report data, AD-23) into a
// Value tree using json.Decoder.UseNumber (AC24): the library owns
// this decode so that number literals reach Decimal intact — a
// caller-decoded `any`/`map[string]any` would arrive with every
// literal already destroyed by encoding/json's default float64
// conversion, which is exactly why Render takes Data ([]byte), never a
// decoded value (AC24).
func DecodeData(d []byte) (Value, error) {
	return decodeJSON(d, "report data")
}

// DecodeParams parses p exactly as DecodeData parses d — through the
// SAME json.NewDecoder/UseNumber call (AC20, D-1.7.5), never a second
// one — but every error names "params", never "report data" (this
// story's review, Finding 6, Minor): a malformed or trailing-garbage
// Params document was previously reported through DecodeData directly
// and read "folio8: Render: params: bind: invalid JSON report data: …"
// — self-contradictory, since M-6/AC16's whole point is that a value
// sought in params was never sought in report data at all.
func DecodeParams(p []byte) (Value, error) {
	return decodeJSON(p, "params")
}

// decodeJSON is DecodeData's and DecodeParams' single shared
// implementation (AC20: one json.NewDecoder/UseNumber call site, not
// two — see internal/bind/decodeguard_test.go's guard). rootLabel
// names what is being decoded ("report data" or "params") in error
// text only, so a decode failure on one root is never reported as
// belonging to the other.
func decodeJSON(d []byte, rootLabel string) (Value, error) {
	dec := json.NewDecoder(bytes.NewReader(d))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return Value{}, fmt.Errorf("bind: invalid JSON %s: %w", rootLabel, err)
	}
	// QA Finding 4 (this story's review, Major): json.Decoder.Decode
	// stops at the end of the first JSON value and silently discards
	// everything after it — render_entry.go documents Render's "d must
	// be syntactically valid JSON" precondition (AC24), but nothing
	// enforced it. Trailing garbage, or a second concatenated document,
	// previously decoded successfully and rendered the FIRST value,
	// silently ignoring the rest — a plausible-looking, wrong render on
	// untrusted caller input, on a product whose acceptance fixture is
	// a bank statement. dec.Token() returns io.EOF only when the
	// stream is genuinely exhausted; anything else means unconsumed
	// content remains.
	if _, err := dec.Token(); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("unexpected additional JSON content")
		}
		return Value{}, fmt.Errorf("bind: trailing data after the JSON %s's single top-level value: %w", rootLabel, err)
	}
	return valueFromAny(v), nil
}

func valueFromAny(v interface{}) Value {
	switch x := v.(type) {
	case nil:
		return Value{Kind: KindNull}
	case bool:
		return Value{Kind: KindBool, Bool: x}
	case json.Number:
		return Value{Kind: KindNumber, Num: string(x)}
	case string:
		return Value{Kind: KindString, Str: x}
	case []interface{}:
		arr := make([]Value, 0, len(x))
		for _, e := range x {
			arr = append(arr, valueFromAny(e))
		}
		return Value{Kind: KindArray, Arr: arr}
	case map[string]interface{}:
		// D-1.3.5/NFR1.d bans ranging a map anywhere under internal/;
		// the escape hatch (sorted keys, then index) is used even
		// though object-key order is not otherwise significant here —
		// consistent with internal/template's rawFromAny, which faces
		// the exact same decoded shape.
		obj := make(map[string]Value, len(x))
		for _, k := range slices.Sorted(maps.Keys(x)) {
			obj[k] = valueFromAny(x[k])
		}
		return Value{Kind: KindObject, Obj: obj}
	default:
		panic(fmt.Sprintf("bind: DecodeData: unexpected decoded type %T", v))
	}
}
