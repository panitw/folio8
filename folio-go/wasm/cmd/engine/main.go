//go:build js && wasm

package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"syscall/js"
	"time"

	folio8 "github.com/panitw/folio8/folio-go"
	"github.com/panitw/folio8/folio-go/fonts"
	"github.com/panitw/folio8/folio-go/internal/designer"
	"github.com/panitw/folio8/folio-go/internal/text"
	"github.com/panitw/folio8/folio-go/internal/wasm"
)

// request is intentionally byte-oriented. The JavaScript boundary never
// reads document values as JS numbers or recreates document fields.
type request struct {
	Operation      string `json:"operation"`
	PayloadBase64  string `json:"payloadBase64,omitempty"`
	TemplateBase64 string `json:"templateBase64,omitempty"`
	DataBase64     string `json:"dataBase64,omitempty"`
	ParamsBase64   string `json:"paramsBase64,omitempty"`
}

type response struct {
	GroupMove                  *wasm.GroupMoveResult            `json:"groupMove,omitempty"`
	OK                         bool                             `json:"ok"`
	Snapshot                   wasm.Snapshot                    `json:"snapshot,omitempty"`
	BytesBase64                string                           `json:"bytesBase64,omitempty"`
	DiagnosticCode             string                           `json:"diagnosticCode,omitempty"`
	Message                    string                           `json:"message,omitempty"`
	ElementID                  string                           `json:"elementId,omitempty"`
	DataPath                   string                           `json:"dataPath,omitempty"`
	DictionarySHA256           string                           `json:"dictionarySha256,omitempty"`
	PDFSHA256                  string                           `json:"pdfSha256,omitempty"`
	PreviewIdentity            string                           `json:"previewIdentity,omitempty"`
	RenderRevision             uint64                           `json:"renderRevision,omitempty"`
	ParameterReferences        *[]string                        `json:"parameterReferences,omitempty"`
	ParameterReferenceRevision uint64                           `json:"parameterReferenceRevision,omitempty"`
	TableColumns               *designer.TableColumnsProjection `json:"tableColumns,omitempty"`
	TableColumnsRevision       uint64                           `json:"tableColumnsRevision,omitempty"`
	// Diagnostics is deliberately not omitempty: an otherwise successful
	// render has the same closed response shape whether it has zero warnings
	// or many. JavaScript treats [] as evidence, while a missing/null field is
	// a protocol violation.
	Diagnostics []diagnostic `json:"diagnostics"`
	// STORY 13.3 — THESE TWO FOLLOW `Diagnostics`, NOT THEIR OTHER NEIGHBOURS.
	// A render that took under a millisecond reports `0 ms`, and `omitempty`
	// would erase exactly that answer: the browser would read "the engine said
	// nothing" from a number the engine did say, and the evidence rail would
	// silently withhold a true fact about a very fast render.
	ElapsedMs int64  `json:"elapsedMs"`
	Version   string `json:"version"`
}

type diagnostic struct {
	Severity  string `json:"severity"`
	Code      string `json:"code"`
	ElementID string `json:"elementId"`
	DataPath  string `json:"dataPath"`
	Message   string `json:"message"`
}

// elapsedClock is the engine's render-elapsed clock: nanoseconds since the
// shell started, read from the monotonic clock. internal/wasm may not import
// `time` (AD-1's forbidden-import rule covers everything under internal/), so
// this shell, outside internal/, is the one place that reads it. Nothing it
// returns reaches a rendered byte.
func elapsedClock() func() int64 {
	origin := time.Now()
	return func() int64 { return time.Since(origin).Nanoseconds() }
}

// maxInstalledFaceBytes bounds ONE face the browser hands the host. It is
// deliberately NOT MAX_ENGINE_PAYLOAD_BYTES and deliberately not on the
// request envelope at all: the CJK face is 10,595,932 raw bytes, over the
// 8 MiB the base64 envelope admits on both sides, and widening that bound
// would relax every operation that rides it. This is a second, wider door
// for one shape of input — raw bytes, no base64 — modelled on
// wasm/cmd/render/main.go's own bytesArg host.
//
// 64 MiB is a backstop against an absurd allocation, not a budget: the
// largest face this repository has ever shipped is a sixth of it.
const maxInstalledFaceBytes = 64 << 20

// maxInstalledFaceNameBytes bounds the face name, in BYTES.
//
// ⚠ IT IS NOT "THE SAME BOUND THE BROWSER APPLIES", and an earlier version of
// this comment said it was. engine-protocol.ts bounds a face name by
// MAX_CANVAS_PROPERTY_STRING, which is 512, and it counts UTF-16 code units
// where this counts UTF-8 bytes — so the two are different quantities even at
// equal numbers, and a name could satisfy either and not the other. Matching
// the number is what keeps every name the browser admits admissible here: 512
// bytes is at least 512 UTF-16 units' worth for ASCII and more than the
// browser's own limit can encode for anything shorter, so this never refuses a
// name the browser let through.
//
// It is a transport backstop either way, not a rule about names. The names
// that actually arrive are FontSet keys Go itself wrote into a refusal, the
// longest of which is twenty characters.
const maxInstalledFaceNameBytes = 512

func main() {
	// THE SHELL DECIDES WHAT THE ENGINE EMBEDS, and this line is where the
	// designer's build states it (spec-deferred-offline-cache, CAP-6).
	// Compiled `-tags nocjkface` — see ENGINE_BUILD_FLAGS in
	// folio-designer/scripts/wasm-vcs-stamp.mjs — fonts.Shipped() is TEN
	// faces here and the CJK face arrives later through installFace.
	// Compiled without the tag this is the ordinary eleven, and the engine
	// behaves exactly as it always has.
	engine := wasm.NewEngine(elapsedClock(), fonts.Shipped())
	handle := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) != 1 || args[0].Type() != js.TypeString {
			return marshal(response{DiagnosticCode: "WASM_PROTOCOL_INVALID", Message: "expected one JSON request string"})
		}
		var in request
		if err := json.Unmarshal([]byte(args[0].String()), &in); err != nil {
			return marshal(response{DiagnosticCode: "WASM_PROTOCOL_INVALID", Message: "malformed request"})
		}
		out := dispatch(engine, in)
		return marshal(out)
	})
	// installFace IS A SECOND ENTRY POINT, NOT A SECOND PROTOCOL. It takes
	// a face name and a Uint8Array and answers with the SAME `response`
	// JSON string `handle` answers with, so the worker parses one shape and
	// the browser-side protocol gains one operation rather than one channel.
	//
	// ⚠ RAW BYTES, BY DESIGN. The face is 10.11 MiB; base64 would put a
	// ~13.5 MiB intermediate string through a `String.fromCharCode` loop on
	// the worker thread and would still be refused by the 8 MiB envelope
	// bound at both ends. js.CopyBytesToGo copies the Uint8Array straight
	// into Go memory — the shape wasm/cmd/render/main.go has used since it
	// was written — and `handle`'s envelope, its operations and its 8 MiB
	// bound are untouched.
	installFace := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) != 2 || args[0].Type() != js.TypeString || args[1].Type() != js.TypeObject {
			return marshal(response{DiagnosticCode: "WASM_PROTOCOL_INVALID", Message: "expected a face name and its bytes"})
		}
		// ⚠ Uint8Array, CHECKED BY IDENTITY, BECAUSE js.TypeObject IS NOT ENOUGH.
		// An ArrayBuffer is also a js.TypeObject and also carries a numeric
		// `byteLength`, and js.CopyBytesToGo PANICS on one — which in js/wasm
		// takes the whole engine instance down rather than returning the
		// diagnostic this entry point promises three lines above. A panic is
		// not a refusal: the worker would report a dead host for an input the
		// protocol can perfectly well describe as invalid.
		//
		// `InstanceOf` and not the constructor's `name`: a constructor is a JS
		// FUNCTION, so `Get("constructor").Type()` is js.TypeFunction and a
		// js.TypeObject test on it rejects every legitimate call — which is
		// exactly what an earlier version of this guard did, refusing the one
		// input it exists to admit.
		uint8Array := js.Global().Get("Uint8Array")
		if uint8Array.Type() != js.TypeFunction || !args[1].InstanceOf(uint8Array) {
			return marshal(response{DiagnosticCode: "WASM_PROTOCOL_INVALID", Message: "expected a face name and its bytes"})
		}
		name := args[0].String()
		if name == "" || len(name) > maxInstalledFaceNameBytes {
			return marshal(failure("WASM_INPUT_INVALID", errors.New("face name is empty or over its bound")))
		}
		length := args[1].Get("byteLength")
		if length.Type() != js.TypeNumber {
			return marshal(response{DiagnosticCode: "WASM_PROTOCOL_INVALID", Message: "expected a face name and its bytes"})
		}
		// THE LENGTH IS CHECKED BEFORE THE BUFFER IS ALLOCATED, for the
		// reason decodeBase64Bounded checks before DecodeString does.
		size := length.Int()
		if size <= 0 || size > maxInstalledFaceBytes {
			return marshal(failure("WASM_INPUT_INVALID", fmt.Errorf("face bytes exceed %d bytes", maxInstalledFaceBytes)))
		}
		face := make([]byte, size)
		// THE COPY'S OWN COUNT IS THE EVIDENCE, and discarding it was a bug:
		// a short copy leaves the tail of `face` as zeros, and a zero-padded
		// font is a face that installs cleanly and then fails somewhere far
		// from here.
		if copied := js.CopyBytesToGo(face, args[1]); copied != size {
			return marshal(failure("WASM_INPUT_INVALID", fmt.Errorf("copied %d of %d face bytes", copied, size)))
		}
		if err := engine.InstallFace(name, face); err != nil {
			return marshal(engineFailure(err))
		}
		// The snapshot is the CURRENT one, unchanged and unadvanced: an
		// install is not an edit. The browser needs it only because every
		// successful response on this protocol carries one.
		return marshal(response{OK: true, Snapshot: engine.Snapshot()})
	})
	js.Global().Set("Folio8WasmHost", js.ValueOf(map[string]any{"handle": handle, "installFace": installFace}))
	select {}
}

func dispatch(engine *wasm.Engine, in request) response {
	decode := func() ([]byte, error) {
		return decodeBase64Bounded(in.PayloadBase64, 8<<20)
	}
	switch in.Operation {
	case "offline-audit":
		return response{OK: true, DictionarySHA256: text.DictionarySHA256()}
	case "initialize", "load":
		payload, err := decode()
		if err != nil {
			return failure("WASM_INPUT_INVALID", err)
		}
		var snapshot wasm.Snapshot
		if in.Operation == "initialize" {
			snapshot, err = engine.Initialize(payload)
		} else {
			snapshot, err = engine.Load(payload)
		}
		if err != nil {
			return engineFailure(err)
		}
		return response{OK: true, Snapshot: snapshot}
	case "snapshot":
		return response{OK: true, Snapshot: engine.Snapshot()}
	case "parameter-references":
		if in.PayloadBase64 != "" || in.TemplateBase64 != "" || in.DataBase64 != "" || in.ParamsBase64 != "" {
			return failure("WASM_INPUT_INVALID", errors.New("parameter references require no byte inputs"))
		}
		references, revision, err := engine.ParameterReferences()
		if err != nil {
			return engineFailure(err)
		}
		return response{OK: true, Snapshot: engine.Snapshot(), ParameterReferences: &references, ParameterReferenceRevision: revision}
	case "stand-in-data":
		if in.PayloadBase64 != "" || in.TemplateBase64 != "" || in.DataBase64 != "" || in.ParamsBase64 != "" {
			return failure("WASM_INPUT_INVALID", errors.New("stand-in data requires no byte inputs"))
		}
		data, err := engine.StandInData()
		if err != nil {
			return engineFailure(err)
		}
		return response{OK: true, Snapshot: engine.Snapshot(), BytesBase64: base64.StdEncoding.EncodeToString(data)}
	case "group-move-preview":
		if in.TemplateBase64 != "" || in.DataBase64 != "" || in.ParamsBase64 != "" {
			return failure("WASM_INPUT_INVALID", errors.New("group move preview requires only a movement payload"))
		}
		payload, err := decode()
		if err != nil {
			return failure("WASM_INPUT_INVALID", err)
		}
		result, err := engine.GroupMovePreview(payload)
		if err != nil {
			return engineFailure(err)
		}
		return response{OK: true, Snapshot: engine.Snapshot(), GroupMove: &result}
	case "table-columns":
		if in.TemplateBase64 != "" || in.DataBase64 != "" || in.ParamsBase64 != "" {
			return failure("WASM_INPUT_INVALID", errors.New("table columns require exactly one selected table id"))
		}
		payload, err := decode()
		if err != nil {
			return failure("WASM_INPUT_INVALID", err)
		}
		var selection struct {
			ID string `json:"id"`
		}
		decoder := json.NewDecoder(strings.NewReader(string(payload)))
		decoder.DisallowUnknownFields()
		var trailing any
		if decoder.Decode(&selection) != nil || decoder.Decode(&trailing) != io.EOF || selection.ID == "" || len(selection.ID) > 128 {
			return failure("WASM_INPUT_INVALID", errors.New("table columns require one selected table id"))
		}
		result, err := engine.TableColumns(selection.ID)
		if err != nil {
			return engineFailure(err)
		}
		return response{OK: true, Snapshot: engine.Snapshot(), TableColumns: &result.Table, TableColumnsRevision: result.Revision}
	case "validate":
		snapshot, err := engine.Validate()
		if err != nil {
			return engineFailure(err)
		}
		return response{OK: true, Snapshot: snapshot}
	case "serialize":
		bytes, snapshot, err := engine.Serialize()
		if err != nil {
			return engineFailure(err)
		}
		return response{OK: true, Snapshot: snapshot, BytesBase64: base64.StdEncoding.EncodeToString(bytes)}
	case "command":
		payload, err := decode()
		if err != nil {
			return failure("WASM_INPUT_INVALID", err)
		}
		snapshot, err := engine.Apply(payload)
		if err != nil {
			return engineFailure(err)
		}
		return response{OK: true, Snapshot: snapshot}
	case "asset":
		payload, err := decode()
		if err != nil {
			return failure("WASM_INPUT_INVALID", err)
		}
		bytes, snapshot, err := engine.AssetBytes(string(payload))
		if err != nil {
			return engineFailure(err)
		}
		return response{OK: true, Snapshot: snapshot, BytesBase64: base64.StdEncoding.EncodeToString(bytes)}
	case "undo":
		snapshot, err := engine.Undo()
		if err != nil {
			return engineFailure(err)
		}
		return response{OK: true, Snapshot: snapshot}
	case "redo":
		snapshot, err := engine.Redo()
		if err != nil {
			return engineFailure(err)
		}
		return response{OK: true, Snapshot: snapshot}
	case "render":
		if in.PayloadBase64 != "" || in.TemplateBase64 == "" || in.DataBase64 == "" || in.ParamsBase64 == "" {
			return failure("WASM_INPUT_INVALID", errors.New("render requires exactly three byte inputs"))
		}
		decodePart := func(value string) ([]byte, error) {
			return decodeBase64Bounded(value, 8<<20)
		}
		template, err := decodePart(in.TemplateBase64)
		if err != nil {
			return failure("WASM_INPUT_INVALID", err)
		}
		data, err := decodePart(in.DataBase64)
		if err != nil {
			return failure("WASM_INPUT_INVALID", err)
		}
		params, err := decodePart(in.ParamsBase64)
		if err != nil {
			return failure("WASM_INPUT_INVALID", err)
		}
		pdf, rendered, err := engine.Render(template, data, params)
		if err != nil {
			return engineFailure(err)
		}
		if len(pdf) > 32<<20 {
			return failure("WASM_OUTPUT_INVALID", errors.New("rendered PDF exceeds 32 MiB"))
		}
		return response{OK: true, Snapshot: engine.Snapshot(), BytesBase64: base64.StdEncoding.EncodeToString(pdf), PDFSHA256: rendered.PDFSHA256, PreviewIdentity: rendered.Identity, RenderRevision: rendered.Revision, Diagnostics: boundedDiagnostics(rendered.Diagnostics), ElapsedMs: rendered.ElapsedMs, Version: rendered.Version}
	case "identity":
		if in.PayloadBase64 != "" || in.TemplateBase64 != "" || in.DataBase64 == "" || in.ParamsBase64 == "" {
			return failure("WASM_INPUT_INVALID", errors.New("identity requires exactly two byte inputs"))
		}
		data, err := decodeBase64Bounded(in.DataBase64, 8<<20)
		if err != nil {
			return failure("WASM_INPUT_INVALID", err)
		}
		params, err := decodeBase64Bounded(in.ParamsBase64, 8<<20)
		if err != nil {
			return failure("WASM_INPUT_INVALID", err)
		}
		identity, revision, err := engine.PreviewIdentity(data, params)
		if err != nil {
			return engineFailure(err)
		}
		return response{OK: true, Snapshot: engine.Snapshot(), PreviewIdentity: identity, RenderRevision: revision}
	default:
		return response{DiagnosticCode: "WASM_OPERATION_UNKNOWN", Message: "unknown operation"}
	}
}

func failure(code string, err error) response {
	return response{DiagnosticCode: code, Message: "The engine request was invalid"}
}

func engineFailure(err error) response {
	if errors.Is(err, wasm.ErrNoUndo) {
		return response{DiagnosticCode: "UNDO_UNAVAILABLE", Message: "Nothing to undo"}
	}
	if errors.Is(err, wasm.ErrNoRedo) {
		return response{DiagnosticCode: "REDO_UNAVAILABLE", Message: "Nothing to redo"}
	}
	var componentErr *designer.ComponentCommandError
	if errors.As(err, &componentErr) {
		return response{
			DiagnosticCode: "COMPONENT_INVALID",
			Message:        bounded(componentErr.Message, 512),
			ElementID:      bounded(componentErr.ElementID, 128),
			DataPath:       bounded(componentErr.DataPath, 256),
		}
	}
	var renderErr *folio8.RenderError
	if errors.As(err, &renderErr) {
		diagnostic := renderErr.Diagnostic
		return response{
			DiagnosticCode: diagnostic.Code,
			Message:        reportableMessage(diagnostic.Code, diagnostic.Message),
			ElementID:      bounded(diagnostic.ElementID, 128),
			DataPath:       bounded(diagnostic.DataPath, 256),
		}
	}
	message := bounded(err.Error(), 512)
	if strings.HasPrefix(message, "folio8: page.") || strings.HasPrefix(message, "width") || strings.HasPrefix(message, "height") {
		path := "page.setup"
		for _, candidate := range []string{"page.width", "page.height", "page.margin.top", "page.margin.right", "page.margin.bottom", "page.margin.left", "page.size", "page.orientation"} {
			if strings.Contains(message, candidate) {
				path = candidate
				break
			}
		}
		return response{DiagnosticCode: "PAGE_SETUP_INVALID", Message: message, DataPath: path}
	}
	// The engine authored this text about a template the caller already holds.
	// Withholding it left the panel with nothing to act on, so report it
	// bounded, exactly as an ordinary render diagnostic's message is reported.
	return response{DiagnosticCode: "ENGINE_REJECTED", Message: message}
}

// reportableMessage decides whether a Diagnostic's own message reaches the
// caller. It does for every engine-authored failure. It does not for a
// malformed template: that message quotes the offending document back, so a
// large or hostile one would be reflected instead of described.
func reportableMessage(code, message string) string {
	if code == folio8.DiagCodeTemplateMalformed {
		return "The template could not be processed"
	}
	return bounded(message, 512)
}

func bounded(value string, max int) string {
	if len(value) > max {
		return value[:max]
	}
	return value
}
func boundedDiagnostics(values []folio8.Diagnostic) []diagnostic {
	if len(values) == 0 {
		return []diagnostic{}
	}
	if len(values) > 256 {
		values = values[:256]
	}
	out := make([]diagnostic, 0, len(values))
	for _, value := range values {
		out = append(out, diagnostic{Severity: strings.ToLower(value.Severity.String()), Code: bounded(value.Code, 96), ElementID: bounded(value.ElementID, 128), DataPath: bounded(value.DataPath, 256), Message: bounded(value.Message, 512)})
	}
	return out
}

// decodeBase64Bounded checks decoded bytes before DecodeString allocates. A
// base64 transport string is larger than its raw bytes, so comparing its text
// length to a raw-byte limit both rejects valid inputs and obscures the real
// memory bound.
func decodeBase64Bounded(value string, max int) ([]byte, error) {
	if len(value)%4 != 0 {
		return nil, errors.New("malformed base64 payload")
	}
	padding := 0
	if strings.HasSuffix(value, "==") {
		padding = 2
	} else if strings.HasSuffix(value, "=") {
		padding = 1
	}
	decoded := len(value)/4*3 - padding
	if decoded < 0 || decoded > max {
		return nil, fmt.Errorf("payload exceeds %d bytes", max)
	}
	return base64.StdEncoding.DecodeString(value)
}
func marshal(out response) string { bytes, _ := json.Marshal(out); return string(bytes) }
