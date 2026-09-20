//go:build js && wasm

// Command render is folio-js's engine: a stateless, render-oriented js/wasm
// host over the public folio8 API. Unlike ../engine (the designer's stateful
// Load/Apply loop), it holds nothing between calls, caps no input beyond what
// wasm memory allows, and compiles in no fonts — every call receives the
// caller's font set.
//
// It registers globalThis.Folio8RenderHost with four functions and the engine
// version. Byte inputs are Uint8Arrays copied with js.CopyBytesToGo; output
// bytes are copied back with js.CopyBytesToJS. Each function returns
// { envelope, bytes? }: parse sets bytes to the canonical template bytes and
// render sets it to the PDF. envelope is a JSON string:
//
//	{"ok":true,  "diagnostics"?: [...], "references"?: [...]}
//	{"ok":false, "error": {"diagnostic": {...}} | {"message": "..."}}
//
// Diagnostics keep Go's order and exact strings; severity is lowercased. A
// panic inside a call is recovered and returned as an error envelope, so the
// Go program, and the host every caller shares, stays alive.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"syscall/js"

	folio8 "github.com/panitw/folio8/folio-go"
)

type diagnostic struct {
	Severity  string `json:"severity"`
	Code      string `json:"code"`
	ElementID string `json:"elementId"`
	DataPath  string `json:"dataPath"`
	Message   string `json:"message"`
}

type failure struct {
	Diagnostic *diagnostic `json:"diagnostic,omitempty"`
	Message    string      `json:"message,omitempty"`
}

type envelope struct {
	OK          bool          `json:"ok"`
	Diagnostics *[]diagnostic `json:"diagnostics,omitempty"`
	References  *[]string     `json:"references,omitempty"`
	Error       *failure      `json:"error,omitempty"`
}

func main() {
	host := map[string]any{
		// parse(template) -> { envelope, bytes: canonical template bytes }
		"parse": guarded(func(args []js.Value) any {
			tpl, err := parse(args, 1)
			if err != nil {
				return reply(fail(err), nil, "")
			}
			canonical, err := folio8.SerializeTemplate(tpl)
			if err != nil {
				return reply(fail(err), nil, "")
			}
			return reply(envelope{OK: true}, canonical, "bytes")
		}),
		// render(template, data, params|null, fontNames, fontBytes, fallback|null) -> { envelope, bytes: pdf }
		"render": guarded(func(args []js.Value) any {
			tpl, err := parse(args, 6)
			if err != nil {
				return reply(fail(err), nil, "")
			}
			mode, err := faceFallback(args[5])
			if err != nil {
				return reply(fail(err), nil, "")
			}
			res, err := folio8.Render(tpl, folio8.Data(bytesArg(args[1])), params(args[2]), fontSet(args[3], args[4]), mode)
			if err != nil {
				return reply(fail(err), nil, "")
			}
			diags := convert(res.Diagnostics)
			return reply(envelope{OK: true, Diagnostics: &diags}, res.Bytes, "bytes")
		}),
		// validate(templateBytes, data, params|null, fontNames, fontBytes, fallback|null) -> { envelope }
		"validate": guarded(func(args []js.Value) any {
			if len(args) != 6 {
				return reply(fail(errArity), nil, "")
			}
			mode, err := faceFallback(args[5])
			if err != nil {
				return reply(fail(err), nil, "")
			}
			found, err := folio8.Validate(bytesArg(args[0]), folio8.Data(bytesArg(args[1])), params(args[2]), fontSet(args[3], args[4]), mode)
			if err != nil {
				return reply(fail(err), nil, "")
			}
			diags := convert(found)
			return reply(envelope{OK: true, Diagnostics: &diags}, nil, "")
		}),
		// parameterReferences(template) -> { envelope }
		"parameterReferences": guarded(func(args []js.Value) any {
			tpl, err := parse(args, 1)
			if err != nil {
				return reply(fail(err), nil, "")
			}
			refs, err := folio8.ParameterReferences(tpl)
			if err != nil {
				return reply(fail(err), nil, "")
			}
			if refs == nil {
				refs = []string{}
			}
			return reply(envelope{OK: true, References: &refs}, nil, "")
		}),
		"version": folio8.Version,
	}
	js.Global().Set("Folio8RenderHost", js.ValueOf(host))
	select {}
}

// guarded wraps a handler so a panic becomes an error envelope instead of
// ending the Go program.
func guarded(handler func(args []js.Value) any) js.Func {
	return js.FuncOf(func(_ js.Value, args []js.Value) (result any) {
		defer func() {
			if recovered := recover(); recovered != nil {
				result = reply(envelope{Error: &failure{Message: fmt.Sprintf("folio8 render host: panic: %v", recovered)}}, nil, "")
			}
		}()
		return handler(args)
	})
}

var errArity = errors.New("folio8 render host: wrong number of arguments")

func parse(args []js.Value, arity int) (*folio8.Template, error) {
	if len(args) != arity {
		return nil, errArity
	}
	return folio8.ParseTemplate(bytesArg(args[0]))
}

// bytesArg copies a Uint8Array into Go memory.
func bytesArg(v js.Value) []byte {
	out := make([]byte, v.Get("byteLength").Int())
	js.CopyBytesToGo(out, v)
	return out
}

// params maps JavaScript null/undefined to Go's nil Params.
func params(v js.Value) folio8.Params {
	if v.IsNull() || v.IsUndefined() {
		return nil
	}
	return folio8.Params(bytesArg(v))
}

// errFallback names a sixth argument this host cannot read as a
// selector.
var errFallback = errors.New("folio8 render host: the face-fallback argument must be null, undefined or a number")

// faceFallback maps the host's sixth argument onto the engine's
// selector. null and undefined are STRICT, so a caller that passes
// nothing keeps the behaviour every call had before the argument
// existed.
//
// ⚠ THE TYPE IS CHECKED BEFORE .Int() IS CALLED. js.Value.Int() PANICS
// on a value that is not a number, and a panic here is recovered into
// an opaque "panic:" envelope instead of the binding's own error — a
// caller passing a string would be told the engine crashed rather than
// that their argument is wrong.
func faceFallback(v js.Value) (folio8.FaceFallback, error) {
	if v.IsNull() || v.IsUndefined() {
		return folio8.FaceFallbackStrict, nil
	}
	if v.Type() != js.TypeNumber {
		return folio8.FaceFallbackStrict, errFallback
	}
	switch v.Int() {
	case 0:
		return folio8.FaceFallbackStrict, nil
	case 1:
		return folio8.FaceFallbackSubstitute, nil
	default:
		return folio8.FaceFallbackStrict, errFallback
	}
}

func fontSet(names, faces js.Value) folio8.FontSet {
	n := names.Length()
	set := make(folio8.FontSet, n)
	for i := 0; i < n; i++ {
		set[names.Index(i).String()] = bytesArg(faces.Index(i))
	}
	return set
}

func convert(values []folio8.Diagnostic) []diagnostic {
	out := make([]diagnostic, 0, len(values))
	for _, v := range values {
		out = append(out, toJSON(v))
	}
	return out
}

func toJSON(v folio8.Diagnostic) diagnostic {
	return diagnostic{Severity: strings.ToLower(v.Severity.String()), Code: v.Code, ElementID: v.ElementID, DataPath: v.DataPath, Message: v.Message}
}

func fail(err error) envelope {
	var renderErr *folio8.RenderError
	if errors.As(err, &renderErr) {
		d := toJSON(renderErr.Diagnostic)
		return envelope{Error: &failure{Diagnostic: &d}}
	}
	return envelope{Error: &failure{Message: err.Error()}}
}

func reply(out envelope, payload []byte, key string) any {
	encoded, err := json.Marshal(out)
	if err != nil {
		encoded, _ = json.Marshal(envelope{Error: &failure{Message: err.Error()}})
	}
	result := map[string]any{"envelope": string(encoded)}
	if key != "" {
		dst := js.Global().Get("Uint8Array").New(len(payload))
		js.CopyBytesToJS(dst, payload)
		result[key] = dst
	}
	return js.ValueOf(result)
}
