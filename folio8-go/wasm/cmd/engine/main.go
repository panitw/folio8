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

	folio8 "github.com/panitw/folio8/folio8-go"
	"github.com/panitw/folio8/folio8-go/internal/designer"
	"github.com/panitw/folio8/folio8-go/internal/text"
	"github.com/panitw/folio8/folio8-go/internal/wasm"
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

func main() {
	engine := wasm.NewEngine(elapsedClock())
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
	js.Global().Set("Folio8WasmHost", js.ValueOf(map[string]any{"handle": handle}))
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
