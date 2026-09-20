//go:build cgo

// Command folio8 is the engine as a native library: a stateless C ABI over
// the public folio8 API, built with `-buildmode=c-shared`. It is what
// folio-dotnet binds with plain DllImport, because no wasm runtime targets
// .NET Framework 4.6.
//
// The ABI is specified in ../../README.md, which is the written contract this
// file and every caller share. In outline:
//
//   - Inputs are pointer plus length byte buffers. A font set arrives as ONE
//     buffer holding a length-prefixed sequence of name/bytes pairs. A
//     duplicate face name is refused rather than resolved.
//   - Every call returns a status int32 and writes three out-parameters: an
//     allocation token, a result buffer pointer, and that buffer's length.
//   - The engine owns what it allocates; the caller returns it with exactly
//     one folio8_free(token). A second free, or an unknown token, is refused
//     with FOLIO8_ERROR_UNKNOWN_FREE — never undefined behaviour.
//   - Nothing is stored between calls. The only global state is the
//     allocation table, and folio8_allocation_count() reports its size so a
//     caller can prove it does not grow.
//   - A panic inside a call is recovered and reported as a status, so the
//     host process stays alive.
//
// Like folio-go/wasm/cmd/render, this entry compiles in no fonts and uses
// only the public API; imports_test.go holds it to that.
package main

/*
#include <stdlib.h>
#include <stdint.h>
*/
import "C"

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sync"
	"unsafe"

	folio8 "github.com/panitw/folio8/folio-go"
)

// Status codes. Keep in step with README.md and with Native.Status in
// folio-dotnet/src/Folio8/Native.cs.
const (
	statusOK               = 0 // the call succeeded; the result buffer is an ok frame
	statusErrorDiagnostic  = 1 // the engine returned a *RenderError; the frame carries its Diagnostic
	statusErrorMessage     = 2 // the engine returned a plain error; the frame carries its message
	statusErrorArgument    = 3 // the call itself was malformed; NO buffer is allocated and no token is issued
	statusErrorPanic       = 4 // a panic was recovered; the frame carries its message
	statusErrorUnknownFree = 5 // folio8_free was given a token this library did not issue, or has already freed
)

// Result frame kinds, the first byte of every result buffer.
const (
	kindOK              = 1
	kindErrorDiagnostic = 2
	kindErrorMessage    = 3
)

// Severity bytes. The unset zero value is deliberately not representable,
// here or in the managed binding.
const (
	severityWarning = 1
	severityError   = 2
)

// The ABI's own C types, aliased so the package's tests can drive these entry
// points directly. `go test` does not support cgo in a _test.go file, and an
// alias declared HERE is an ordinary package-level name every other file in
// the package can use — which is what lets abi_test.go execute the argument
// and panic statuses rather than describe them. These are aliases, not
// defined types: cI32 and C.int32_t are the same type.
type (
	cU64 = C.uint64_t
	cI32 = C.int32_t
)

// abiVersion is the shape of everything above: the exports, their parameters,
// the status codes and the frame grammar. A caller checks it ONCE before its
// first call, so a managed assembly paired with a native library from a
// different build refuses to start rather than decoding garbage. Bump it for
// any change a caller would have to be recompiled for.
// Moved 1 -> 2 by spec-font-sources-and-embedding: folio8_render and
// folio8_validate gained a trailing int32 face-fallback selector before
// their out-parameters, which is a signature a caller must be recompiled
// against.
const abiVersion = 2

// maxFrame is the largest result buffer the ABI can describe: the length is
// an int32, so a frame beyond this cannot be reported at all. A document that
// big is refused with a named message rather than handed back with a
// wrapped-around length.
const maxFrame = math.MaxInt32

func main() {}

// ---------------------------------------------------------------------------
// Allocation table
// ---------------------------------------------------------------------------

var (
	allocMu    sync.Mutex
	allocs            = map[uint64]unsafe.Pointer{}
	allocNext  uint64 = 1
	errArgs           = errors.New("folio8 cshared: malformed call")
	errNoFonts        = errors.New("folio8 cshared: malformed font buffer")
)

// keep stores buf in C memory and returns the token that frees it. The
// returned pointer is always non-nil: C.CBytes allocates len+1 bytes, so even
// an empty frame has a distinct address.
func keep(buf []byte) (uint64, unsafe.Pointer, int32) {
	p := C.CBytes(buf)
	allocMu.Lock()
	token := allocNext
	allocNext++
	allocs[token] = p
	allocMu.Unlock()
	return token, p, int32(len(buf))
}

//export folio8_free
func folio8_free(token C.uint64_t) C.int32_t {
	allocMu.Lock()
	p, ok := allocs[uint64(token)]
	if ok {
		delete(allocs, uint64(token))
	}
	allocMu.Unlock()
	if !ok {
		return statusErrorUnknownFree
	}
	C.free(p)
	return statusOK
}

// folio8_allocation_count reports how many tokens are outstanding. It exists
// so a caller can prove free discipline over a loop rather than assert it.
//
//export folio8_allocation_count
func folio8_allocation_count() C.int32_t {
	allocMu.Lock()
	n := len(allocs)
	allocMu.Unlock()
	return C.int32_t(n)
}

// ---------------------------------------------------------------------------
// Frame encoding
// ---------------------------------------------------------------------------

// frame builds one result buffer. An encoding problem is RECORDED rather
// than ignored: severityOf below refuses a Severity that is neither Warning
// nor Error, and reply turns that into a message frame instead of shipping a
// byte the reader cannot interpret.
type frame struct {
	buf []byte
	err error
}

func (f *frame) u8(v byte) { f.buf = append(f.buf, v) }

func (f *frame) u32(v int) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], uint32(v))
	f.buf = append(f.buf, b[:]...)
}

func (f *frame) blob(b []byte) {
	f.u32(len(b))
	f.buf = append(f.buf, b...)
}

func (f *frame) str(s string) {
	f.u32(len(s))
	f.buf = append(f.buf, s...)
}

func (f *frame) diagnostic(d folio8.Diagnostic) {
	switch d.Severity {
	case folio8.SeverityWarning:
		f.u8(severityWarning)
	case folio8.SeverityError:
		f.u8(severityError)
	default:
		// The unset zero value, or a severity added after this ABI was
		// written. Either way it is NOT a warning, and quietly calling it one
		// would be the binding inventing a disposition the engine did not
		// give it.
		if f.err == nil {
			f.err = fmt.Errorf("folio8 cshared: diagnostic %q carries severity %v, which this ABI cannot represent", d.Code, d.Severity)
		}
		f.u8(0)
	}
	f.str(d.Code)
	f.str(d.ElementID)
	f.str(d.DataPath)
	f.str(d.Message)
}

// okReply is the one success shape every call writes: warnings, then
// parameter references, then a payload. A call that produces none of a
// section still writes its count or length as zero, so the reader is the same
// for every function.
func okReply(diags []folio8.Diagnostic, refs []string, payload []byte) (int32, []byte) {
	f := &frame{}
	f.u8(kindOK)
	f.u32(len(diags))
	for _, d := range diags {
		f.diagnostic(d)
	}
	f.u32(len(refs))
	for _, r := range refs {
		f.str(r)
	}
	f.blob(payload)
	if f.err != nil {
		return statusErrorMessage, messageFrame(f.err.Error())
	}
	return statusOK, f.buf
}

// messageFrame is the minimal kind-3 frame: a status's detail with nothing
// else in it.
func messageFrame(message string) []byte {
	f := &frame{}
	f.u8(kindErrorMessage)
	f.str(message)
	return f.buf
}

// failure maps a Go error onto a status and a frame, preserving Go's split:
// a *RenderError carries a Diagnostic; anything else carries only its message.
func failure(err error) (int32, []byte) {
	f := &frame{}
	var renderErr *folio8.RenderError
	if errors.As(err, &renderErr) {
		f.u8(kindErrorDiagnostic)
		f.diagnostic(renderErr.Diagnostic)
		if f.err != nil {
			return statusErrorMessage, messageFrame(f.err.Error())
		}
		return statusErrorDiagnostic, f.buf
	}
	return statusErrorMessage, messageFrame(err.Error())
}

func panicFrame(recovered any) []byte {
	return messageFrame(fmt.Sprintf("folio8 cshared: panic: %v", recovered))
}

// ---------------------------------------------------------------------------
// Input decoding
// ---------------------------------------------------------------------------

// input copies a caller buffer into Go memory. A nil pointer with a zero
// length is an empty buffer; a nil pointer with a positive length, or any
// negative length, is a malformed call.
func input(p unsafe.Pointer, n C.int32_t) ([]byte, error) {
	if n < 0 || (p == nil && n > 0) {
		return nil, errArgs
	}
	if n == 0 {
		return []byte{}, nil
	}
	return C.GoBytes(p, C.int(n)), nil
}

// fontSet decodes the one font buffer: a sequence of
//
//	uint32 nameLen | name (UTF-8) | uint32 faceLen | face bytes
//
// running to the end of the buffer. The engine's FontSet is a map, so the
// ORDER of the pairs does not reach the engine and must not be promised to;
// what the order does decide, under a last-wins decode, is which of two
// entries sharing a name survives. A duplicate is therefore REFUSED as a
// malformed buffer rather than silently resolved.
func fontSet(p unsafe.Pointer, n C.int32_t) (folio8.FontSet, error) {
	raw, err := input(p, n)
	if err != nil {
		return nil, err
	}
	set := folio8.FontSet{}
	for len(raw) > 0 {
		name, rest, ok := takeBlob(raw)
		if !ok {
			return nil, errNoFonts
		}
		face, rest, ok := takeBlob(rest)
		if !ok {
			return nil, errNoFonts
		}
		if _, duplicate := set[string(name)]; duplicate {
			return nil, fmt.Errorf("%w: face %q appears more than once", errNoFonts, string(name))
		}
		set[string(name)] = face
		raw = rest
	}
	return set, nil
}

// faceFallback decodes the selector's one int32. The two values are the
// ABI's own spelling of folio8.FaceFallback, kept in step with README.md
// and with the managed FaceFallback enum in
// folio-dotnet/src/Folio8/FaceFallback.cs.
//
// AN UNKNOWN VALUE IS FOLIO8_ERROR_ARGUMENT, never a clamp to strict: a
// caller who computed a value outside the set has a bug, and rendering
// something plausible for them hides it. Go returns a named error for
// the same input and the JS binding throws; this is the third spelling
// of one contract.
//
// 0 IS STRICT, so a caller that writes a zero — including one porting
// from the pre-ABI-2 signature — gets the behaviour every pre-selector
// call had.
func faceFallback(v C.int32_t) (folio8.FaceFallback, error) {
	switch int32(v) {
	case 0:
		return folio8.FaceFallbackStrict, nil
	case 1:
		return folio8.FaceFallbackSubstitute, nil
	default:
		return folio8.FaceFallbackStrict, errArgs
	}
}

func takeBlob(raw []byte) (blob, rest []byte, ok bool) {
	if len(raw) < 4 {
		return nil, nil, false
	}
	size := int(binary.LittleEndian.Uint32(raw[:4]))
	if size < 0 || len(raw)-4 < size {
		return nil, nil, false
	}
	return raw[4 : 4+size], raw[4+size:], true
}

// ---------------------------------------------------------------------------
// The call wrapper
// ---------------------------------------------------------------------------

// call runs body, recovers any panic, and publishes the outcome through the
// three out-parameters every exported function shares. A status of
// FOLIO8_ERROR_ARGUMENT issues no token and allocates nothing.
func call(outToken *C.uint64_t, outResult *unsafe.Pointer, outLen *C.int32_t, body func() (int32, []byte)) (status C.int32_t) {
	if outToken == nil || outResult == nil || outLen == nil {
		return statusErrorArgument
	}
	*outToken = 0
	*outResult = nil
	*outLen = 0

	var code int32
	var buf []byte
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				code, buf = statusErrorPanic, panicFrame(recovered)
			}
		}()
		code, buf = body()
	}()
	if code == statusErrorArgument {
		return statusErrorArgument
	}
	if len(buf) > maxFrame {
		// The out-parameter is an int32; a longer frame cannot be described,
		// and reporting a wrapped-around length would be worse than refusing.
		code, buf = statusErrorMessage, messageFrame(fmt.Sprintf("folio8 cshared: result of %d bytes exceeds the %d-byte limit this ABI can describe", len(buf), maxFrame))
	}
	token, p, n := keep(buf)
	*outToken = C.uint64_t(token)
	*outResult = p
	*outLen = C.int32_t(n)
	return C.int32_t(code)
}

// ---------------------------------------------------------------------------
// Exports
// ---------------------------------------------------------------------------

// folio8_abi_version reports the shape of this ABI. It allocates nothing and
// cannot fail, so a caller can call it before anything else — which is the
// point: a managed binding checks it ONCE before its first real call, and
// refuses a native library it was not built against instead of decoding a
// frame grammar that has moved under it.
//
//export folio8_abi_version
func folio8_abi_version() C.int32_t {
	return abiVersion
}

// folio8_version writes the engine version string as the frame's payload.
//
//export folio8_version
func folio8_version(outToken *C.uint64_t, outResult *unsafe.Pointer, outLen *C.int32_t) C.int32_t {
	return call(outToken, outResult, outLen, func() (int32, []byte) {
		return okReply(nil, nil, []byte(folio8.Version))
	})
}

// folio8_parse parses template bytes and writes the CANONICAL template bytes
// as the payload, mirroring Go's ParseTemplate followed by SerializeTemplate.
// No handle survives the call: the caller keeps the canonical bytes and hands
// them back to render or parameter references.
//
//export folio8_parse
func folio8_parse(tpl unsafe.Pointer, tplLen C.int32_t, outToken *C.uint64_t, outResult *unsafe.Pointer, outLen *C.int32_t) C.int32_t {
	return call(outToken, outResult, outLen, func() (int32, []byte) {
		b, err := input(tpl, tplLen)
		if err != nil {
			return statusErrorArgument, nil
		}
		parsed, err := folio8.ParseTemplate(b)
		if err != nil {
			return failure(err)
		}
		canonical, err := folio8.SerializeTemplate(parsed)
		if err != nil {
			return failure(err)
		}
		return okReply(nil, nil, canonical)
	})
}

// folio8_render renders a PDF. Warnings reach the frame's diagnostic section;
// an error aborts and nothing is rendered, exactly as in Go.
//
//export folio8_render
func folio8_render(
	tpl unsafe.Pointer, tplLen C.int32_t,
	data unsafe.Pointer, dataLen C.int32_t,
	params unsafe.Pointer, paramsLen C.int32_t,
	fonts unsafe.Pointer, fontsLen C.int32_t,
	fallback C.int32_t,
	outToken *C.uint64_t, outResult *unsafe.Pointer, outLen *C.int32_t,
) C.int32_t {
	return call(outToken, outResult, outLen, func() (int32, []byte) {
		tplBytes, d, p, set, err := renderInputs(tpl, tplLen, data, dataLen, params, paramsLen, fonts, fontsLen)
		if err != nil {
			return statusErrorArgument, nil
		}
		mode, err := faceFallback(fallback)
		if err != nil {
			return statusErrorArgument, nil
		}
		parsed, err := folio8.ParseTemplate(tplBytes)
		if err != nil {
			return failure(err)
		}
		res, err := folio8.Render(parsed, d, p, set, mode)
		if err != nil {
			return failure(err)
		}
		return okReply(res.Diagnostics, nil, res.Bytes)
	})
}

// folio8_validate predicts a render without performing one, returning Go's
// diagnostic slice verbatim. It takes raw template bytes, matching Go.
//
//export folio8_validate
func folio8_validate(
	tpl unsafe.Pointer, tplLen C.int32_t,
	data unsafe.Pointer, dataLen C.int32_t,
	params unsafe.Pointer, paramsLen C.int32_t,
	fonts unsafe.Pointer, fontsLen C.int32_t,
	fallback C.int32_t,
	outToken *C.uint64_t, outResult *unsafe.Pointer, outLen *C.int32_t,
) C.int32_t {
	return call(outToken, outResult, outLen, func() (int32, []byte) {
		tplBytes, d, p, set, err := renderInputs(tpl, tplLen, data, dataLen, params, paramsLen, fonts, fontsLen)
		if err != nil {
			return statusErrorArgument, nil
		}
		mode, err := faceFallback(fallback)
		if err != nil {
			return statusErrorArgument, nil
		}
		found, err := folio8.Validate(tplBytes, d, p, set, mode)
		if err != nil {
			return failure(err)
		}
		return okReply(found, nil, nil)
	})
}

// folio8_parameter_references writes the param names a template reads into
// the frame's reference section.
//
//export folio8_parameter_references
func folio8_parameter_references(tpl unsafe.Pointer, tplLen C.int32_t, outToken *C.uint64_t, outResult *unsafe.Pointer, outLen *C.int32_t) C.int32_t {
	return call(outToken, outResult, outLen, func() (int32, []byte) {
		b, err := input(tpl, tplLen)
		if err != nil {
			return statusErrorArgument, nil
		}
		parsed, err := folio8.ParseTemplate(b)
		if err != nil {
			return failure(err)
		}
		refs, err := folio8.ParameterReferences(parsed)
		if err != nil {
			return failure(err)
		}
		return okReply(nil, refs, nil)
	})
}

func renderInputs(
	tpl unsafe.Pointer, tplLen C.int32_t,
	data unsafe.Pointer, dataLen C.int32_t,
	params unsafe.Pointer, paramsLen C.int32_t,
	fonts unsafe.Pointer, fontsLen C.int32_t,
) ([]byte, folio8.Data, folio8.Params, folio8.FontSet, error) {
	tplBytes, err := input(tpl, tplLen)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	dataBytes, err := input(data, dataLen)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	// ABSENT AND EMPTY ARE THE SAME THING HERE, and that is the engine's
	// rule, not a shortcut: decodeParams (render_entry.go) maps any Params of
	// length zero — nil included — onto the empty params object. So a null
	// pointer and a zero-length buffer are deliberately NOT distinguished,
	// and README.md says so rather than leaving a caller to discover it.
	paramBytes, err := input(params, paramsLen)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	set, err := fontSet(fonts, fontsLen)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return tplBytes, folio8.Data(dataBytes), folio8.Params(paramBytes), set, nil
}
