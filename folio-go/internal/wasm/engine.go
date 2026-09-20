// Package wasm is the designer's session engine around the pure folio8 core.
// It owns mutable browser-session state and exposes only bytes plus a
// deliberately small UI projection. It is internal: the designer's js/wasm
// shell, wasm/cmd/engine, is its one caller, and it is not public API.
//
// It reads no clock. The render-elapsed number comes from the clock the shell
// passes to NewEngine, because `time` is a forbidden import under internal/.
package wasm

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"maps"

	folio8 "github.com/panitw/folio8/folio-go"
	"github.com/panitw/folio8/folio-go/internal/designer"
	"github.com/panitw/folio8/folio-go/internal/fontset"
)

const historyLimit = 100

var ErrNoUndo = errors.New("folio8 wasm: no undo history")
var ErrNoRedo = errors.New("folio8 wasm: no redo history")

// Snapshot is a paint-safe projection, not a .folio schema mirror.
type Snapshot struct {
	DocumentState string                     `json:"documentState"`
	Revision      uint64                     `json:"revision"`
	ByteLength    int                        `json:"byteLength"`
	CanUndo       bool                       `json:"canUndo"`
	CanRedo       bool                       `json:"canRedo"`
	Canvas        *designer.CanvasProjection `json:"canvas,omitempty"`
}

type TableColumnsResult struct {
	Revision uint64                          `json:"revision"`
	Table    designer.TableColumnsProjection `json:"table"`
}

// RenderResult is a deliberately opaque production-render projection. It is
// never a browser document model: callers can only receive the PDF bytes,
// their producer-computed digest, the revision that supplied template bytes,
// bounded diagnostics from the existing production renderer, and — since
// Story 13.3 — two facts about the render that produced them.
//
// NEITHER NEW FIELD IS `omitempty`, and that is the same reason `Diagnostics`
// is not. A render that took under a millisecond reports `0`, and `omitempty`
// would drop it: the browser would then read "the engine did not say" from a
// number the engine did say. `ElapsedMs` is milliseconds because
// `Duration.Seconds()` is a `float64`, which `TestNoFloat64UnderModule` refuses
// anywhere under this module.
type RenderResult struct {
	PDFSHA256   string              `json:"pdfSha256"`
	Identity    string              `json:"identity"`
	Revision    uint64              `json:"revision"`
	Diagnostics []folio8.Diagnostic `json:"diagnostics"`
	ElapsedMs   int64               `json:"elapsedMs"`
	Version     string              `json:"version"`
}

// ParameterReferences exposes only engine-derived display metadata for the
// transient Preview parameter editor. It is neither document state nor a
// template/schema projection.
func (e *Engine) ParameterReferences() ([]string, uint64, error) {
	if e.template == nil {
		return nil, 0, fmt.Errorf("folio8 wasm: no document is loaded")
	}
	references, err := folio8.ParameterReferences(e.template)
	if err != nil {
		return nil, 0, err
	}
	// The browser protocol's closed parameterReferences shape requires an
	// array. A nil Go slice marshals as null and would invalidate an otherwise
	// healthy worker response when the document has no params references.
	out := make([]string, len(references))
	copy(out, references)
	return out, e.revision, nil
}

// StandInData exposes the read-only stand-in data projection for the
// current document: a real JSON document whose value at each referenced
// path is chosen by the expression WRAPPING that path.
//
// It returns BYTES on the existing response envelope, and that is the
// whole point: the browser sends them on the `data` channel it already
// has, so Render is called with genuinely supplied data and its
// semantics, error contract and codes are untouched STRUCTURALLY rather
// than by discipline.
//
// IT RETURNS NO REVISION OF ITS OWN, deliberately, unlike
// ParameterReferences. The dispatch arm answers with the SAME
// engine.Snapshot() every other arm does, and that snapshot's revision
// is the one the browser correlates against — a second copy returned
// here would be the same number arriving twice, and the only caller
// discarded it.
func (e *Engine) StandInData() ([]byte, error) {
	if e.template == nil {
		return nil, fmt.Errorf("folio8 wasm: no document is loaded")
	}
	return designer.StandInData(e.template)
}

// TableColumns exposes one revision-correlated selected-table projection.
// It is intentionally a query, not a browser-side document model.
func (e *Engine) TableColumns(tableID string) (TableColumnsResult, error) {
	if e.template == nil {
		return TableColumnsResult{}, fmt.Errorf("folio8 wasm: no document is loaded")
	}
	table, err := designer.TableColumns(e.template, tableID)
	if err != nil {
		return TableColumnsResult{}, err
	}
	return TableColumnsResult{Revision: e.revision, Table: table}, nil
}

// GroupMovePreview reports the same accepted translation used by Apply without
// changing canonical bytes, revision, or either history branch.
type GroupMoveResult struct {
	Revision uint64 `json:"revision"`
	DX       int64  `json:"dx"`
	DY       int64  `json:"dy"`
}

func (e *Engine) GroupMovePreview(command []byte) (GroupMoveResult, error) {
	if e.template == nil {
		return GroupMoveResult{}, fmt.Errorf("folio8 wasm: no document is loaded")
	}
	if err := e.checkMoveRevision(command); err != nil {
		return GroupMoveResult{}, err
	}
	move, err := designer.PreviewComponentMove(e.template, command, e.faces)
	if err != nil {
		return GroupMoveResult{}, err
	}
	return GroupMoveResult{Revision: e.revision, DX: move.DX, DY: move.DY}, nil
}

// checkMoveRevision is the ONLY place a moveComponents' expectedRevision is
// compared to the revision it was drafted against. group_movement.go requires
// the field to be present and bounded but never asks whether it is current, so
// a move that slips past this function is a move applied to a document the
// author was not looking at.
//
// IT WALKS THE COMMANDS THE COMMAND CARRIES, not the command it was handed. A
// move inside a unit of commands meets exactly this fence, with exactly this
// wording, because the alternative — a fence that only sees a bare move — would
// admit a stale move simply for travelling inside a unit. What a command
// carries is folio8's question to answer, asked through the designer bridge:
// this package must never learn to decode a unit's member list itself, or the
// two can disagree. A command that carries nothing else carries itself, so the
// bare-move case is the same code path and needs no branch.
//
// IT RUNS BEFORE THE COMMAND DOOR HAS JUDGED ANYTHING, which is why a member it
// cannot decode is NOT its to report. Refusing here would put this package's
// unlocated "command is malformed" in front of the member's own located
// refusal, at the only seam the browser ever sees — so a unit's undecodable
// member is skipped and left to the door. Only a command that carries ITSELF
// and does not decode is this function's own malformed-input case, and it
// answers exactly as it always has.
func (e *Engine) checkMoveRevision(command []byte) error {
	carried, unit := designer.CarriedCommands(command)
	for _, member := range carried {
		var intent struct {
			Kind             string  `json:"kind"`
			ExpectedRevision *uint64 `json:"expectedRevision"`
		}
		if err := json.Unmarshal(member, &intent); err != nil {
			if unit {
				continue
			}
			return fmt.Errorf("folio8 wasm: command is malformed")
		}
		if intent.Kind == "moveComponents" && (intent.ExpectedRevision == nil || *intent.ExpectedRevision != e.revision) {
			return fmt.Errorf("folio8 wasm: group move refers to an outdated revision")
		}
	}
	return nil
}

// PreviewIdentity obtains evidence from the current engine-owned canonical
// template and the two raw JSON channels without exposing or parsing the
// template in the browser.
func (e *Engine) PreviewIdentity(data, params []byte) (string, uint64, error) {
	if e.template == nil {
		return "", 0, fmt.Errorf("folio8 wasm: no document is loaded")
	}
	if len(data) == 0 || len(params) == 0 {
		return "", 0, fmt.Errorf("folio8 wasm: identity inputs must be non-empty")
	}
	return designer.PreviewIdentity(e.bytes, folio8.Data(data), folio8.Params(params), e.faces), e.revision, nil
}

// Engine owns one live template and its canonical bytes for one worker.
type Engine struct {
	clock func() int64
	// faces is THE font set this engine measures, paints and renders with —
	// one set, held for the session, read at every one of the seven sites
	// that used to call fonts.Shipped() directly
	// (spec-deferred-offline-cache, CAP-6).
	//
	// IT IS INJECTED, LIKE clock, AND FOR THE SAME REASON. This package is
	// under internal/ and states no opinion about which faces a build
	// embeds; wasm/cmd/engine, the shell, decides — and since that shell is
	// compiled `-tags nocjkface`, what it passes is a TEN-face set. The
	// eleventh arrives later, from the browser, through InstallFace.
	//
	// IT IS ALSO WHY Apply AND GroupMovePreview COST NOTHING EXTRA. Both are
	// hot — Apply fires about every 200 ms while typing — and both used to
	// rebuild the whole shipped map per call. Holding it makes the CJK face
	// a session-lifetime install rather than a per-request payload, which
	// the 8 MiB request envelope forbids in any case.
	faces    folio8.FontSet
	template *folio8.Template
	bytes    []byte
	revision uint64
	canvas   *designer.CanvasProjection
	undo     [][]byte
	redo     [][]byte
}

// NewEngine returns an empty engine. clock reports nanoseconds on any fixed
// origin and is read only to bracket Render; the difference of two readings,
// truncated to whole milliseconds, is RenderResult.ElapsedMs. Nothing it returns reaches a rendered byte.
//
// STORY 13.3 — the clock comes from the caller. AD-1's determinism boundary is
// a directory boundary: the forbidden-import rule
// (lint/internal/rules/forbiddenimports.go) bans `time` everywhere under
// folio-go/internal/, this package included. wasm/cmd/engine, the imperative
// js/wasm shell outside internal/, is the one place that reads a real clock.
// faces is the set this engine measures and renders with. The caller keeps
// no handle on it: the map is copied, so a host that reuses its own
// fonts.Shipped() result cannot mutate the engine's set afterwards, and
// InstallFace cannot write back into the caller's.
func NewEngine(clock func() int64, faces folio8.FontSet) *Engine {
	// maps.Copy, not a range: AD-1/NFR1.d forbids ranging a map value
	// under internal/, and copying every key makes the order irrelevant.
	held := make(folio8.FontSet, len(faces)+1)
	maps.Copy(held, faces)
	return &Engine{clock: clock, faces: held}
}

// InstallFace holds one named face for the rest of this session, merged
// OVER the set NewEngine was given (spec-deferred-offline-cache, CAP-6).
//
// THIS IS THE ONE WAY A FACE ENTERS AFTER CONSTRUCTION, and it exists
// because the designer's engine wasm no longer embeds the CJK face: the
// browser fetches it from the deferred release asset that already carries
// the identical bytes and hands it here, once, the first time a render or
// a projection refuses for want of it.
//
// IT IS IDEMPOTENT AND IT DOES NOT REPROJECT. Installing the same face
// twice is the same state as installing it once, and neither advances the
// revision, touches history, or redraws the canvas: the caller retries the
// request that refused, and that request produces the projection. A
// silent reprojection here would hand the browser a snapshot it never
// asked for and could not correlate.
//
// THE BYTES ARE COPIED. They arrive as a Uint8Array copied into Go memory
// by the shell, and copying again here costs one allocation per SESSION
// while guaranteeing the engine's set can never alias a caller's slice.
//
// ⚠ AND THEY ARE PARSED BEFORE THEY ARE STORED. A truncated, corrupted or
// simply wrong download would otherwise become a permanently installed bad
// face for the whole session — the worker is never restarted — and the
// failure would land deep inside a later render, in a shape the browser's
// absent-face recovery cannot read. Failing on the INSTALL puts it where
// the recovery already handles it: the request rejects, the recovery
// reports that it supplied nothing, and the original TEXT_FACE_ABSENT
// refusal is what the author is told.
func (e *Engine) InstallFace(name string, face []byte) error {
	if name == "" {
		return fmt.Errorf("folio8 wasm: a face must be installed under a name")
	}
	if len(face) == 0 {
		return fmt.Errorf("folio8 wasm: face %q was supplied with no bytes", name)
	}
	if _, err := fontset.New(name, face); err != nil {
		return fmt.Errorf("folio8 wasm: face %q was supplied with bytes that are not a usable font: %w", name, err)
	}
	if e.faces == nil {
		e.faces = folio8.FontSet{}
	}
	e.faces[name] = append([]byte(nil), face...)
	return nil
}

// Initialize and Load parse through the public engine boundary before storing
// a canonical copy. A caller's byte slice is never retained or aliased.
func (e *Engine) Initialize(input []byte) (Snapshot, error) { return e.load(input) }
func (e *Engine) Load(input []byte) (Snapshot, error)       { return e.load(input) }

func (e *Engine) load(input []byte) (Snapshot, error) {
	tpl, err := folio8.ParseTemplate(input)
	if err != nil {
		return Snapshot{}, err
	}
	canonical, err := folio8.SerializeTemplate(tpl)
	if err != nil {
		return Snapshot{}, err
	}
	projection, err := designer.CanvasWithTextPaint(tpl, e.faces)
	if err != nil {
		return Snapshot{}, err
	}
	// Install only after every fallible projection step has completed. Failed
	// Open/Initialize must leave the old template, bytes, snapshot and revision
	// untouched so a later Save cannot serialize rejected input.
	e.template = tpl
	e.bytes = append(e.bytes[:0], canonical...)
	e.canvas = &projection
	e.undo = nil
	e.redo = nil
	e.revision++
	return e.Snapshot(), nil
}

func (e *Engine) Snapshot() Snapshot {
	if e.template == nil {
		return Snapshot{DocumentState: "empty", Revision: e.revision, CanUndo: len(e.undo) > 0, CanRedo: len(e.redo) > 0}
	}
	return Snapshot{DocumentState: "loaded", Revision: e.revision, ByteLength: len(e.bytes), CanUndo: len(e.undo) > 0, CanRedo: len(e.redo) > 0, Canvas: e.canvas}
}

// Serialize returns a copy of canonical bytes. Bytes are the authority across
// the worker boundary; no live document handle is exposed.
func (e *Engine) Serialize() ([]byte, Snapshot, error) {
	if e.template == nil {
		return nil, Snapshot{}, fmt.Errorf("folio8 wasm: no document is loaded")
	}
	return append([]byte(nil), e.bytes...), e.Snapshot(), nil
}

// Render reparses the caller-provided canonical template bytes through the
// production boundary and invokes the one public renderer. The three byte
// channels intentionally remain distinct: data and params must preserve the
// exact-decimal JSON semantics owned by folio8.Render.
func (e *Engine) Render(template, data, params []byte) ([]byte, RenderResult, error) {
	if e.template == nil {
		return nil, RenderResult{}, fmt.Errorf("folio8 wasm: no document is loaded")
	}
	if len(template) == 0 || len(data) == 0 || len(params) == 0 {
		return nil, RenderResult{}, fmt.Errorf("folio8 wasm: render inputs must be non-empty")
	}
	if !bytes.Equal(template, e.bytes) {
		return nil, RenderResult{}, fmt.Errorf("folio8 wasm: render template is not the current canonical revision")
	}
	tpl, err := folio8.ParseTemplate(template)
	if err != nil {
		return nil, RenderResult{}, err
	}
	// THE BRACKET IS THE RENDER AND NOTHING ELSE. It excludes ParseTemplate
	// above, the digest below and PreviewIdentity after it, and it is nowhere
	// near the worker round trip — a number that included postMessage and byte
	// transfer would satisfy the word "elapsed" while describing the browser's
	// transport rather than the engine's work.
	started := e.clock()
	result, err := folio8.Render(tpl, folio8.Data(data), folio8.Params(params), e.faces)
	elapsed := (e.clock() - started) / 1_000_000
	if err != nil {
		return nil, RenderResult{}, err
	}
	pdf := append([]byte(nil), result.Bytes...)
	digest := sha256.Sum256(pdf)
	identity, revision, err := e.PreviewIdentity(data, params)
	if err != nil {
		return nil, RenderResult{}, err
	}
	return pdf, RenderResult{PDFSHA256: fmt.Sprintf("%x", digest), Identity: identity, Revision: revision, Diagnostics: append([]folio8.Diagnostic(nil), result.Diagnostics...), ElapsedMs: elapsed, Version: folio8.Version}, nil
}

// AssetBytes is Story 5.13's per-key paintable-bytes query (D-5.13.2's
// "Producer" clause). It is read-only: it never advances revision or
// touches undo/redo history, and it never reproduces asset lookup/decoding
// rules here — folio8's unexported assetBytes, reached through designer.AssetBytes, owns those.
func (e *Engine) AssetBytes(key string) ([]byte, Snapshot, error) {
	if e.template == nil {
		return nil, Snapshot{}, fmt.Errorf("folio8 wasm: no document is loaded")
	}
	raw, _, err := designer.AssetBytes(e.template, key)
	if err != nil {
		return nil, Snapshot{}, err
	}
	return raw, e.Snapshot(), nil
}

// Validate reparses the engine-owned canonical bytes. It deliberately does
// not reproduce validation rules in the transport layer.
func (e *Engine) Validate() (Snapshot, error) {
	if e.template == nil {
		return Snapshot{}, fmt.Errorf("folio8 wasm: no document is loaded")
	}
	if _, err := folio8.ParseTemplate(e.bytes); err != nil {
		return Snapshot{}, err
	}
	return e.Snapshot(), nil
}

// Apply accepts only opaque, engine-defined committed commands.
func (e *Engine) Apply(command []byte) (Snapshot, error) {
	if e.template == nil {
		return Snapshot{}, fmt.Errorf("folio8 wasm: no document is loaded")
	}
	if err := e.checkMoveRevision(command); err != nil {
		return Snapshot{}, err
	}
	// Apply to a fresh canonical clone. This makes command validation,
	// serialization and projection one transaction rather than relying on a
	// rollback that can itself alter the revision.
	candidate, err := folio8.ParseTemplate(e.bytes)
	if err != nil {
		return Snapshot{}, err
	}
	var commandKind struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(command, &commandKind); err != nil {
		return Snapshot{}, fmt.Errorf("folio8 wasm: command is malformed")
	}
	var projection designer.CanvasProjection
	if commandKind.Kind == "pageSetup" {
		projection, err = designer.ApplyPageSetupCommand(candidate, command)
	} else {
		projection, err = designer.ApplyComponentCommand(candidate, command, e.faces)
	}
	if err != nil {
		return Snapshot{}, err
	}
	canonical, err := folio8.SerializeTemplate(candidate)
	if err != nil {
		return Snapshot{}, err
	}
	// Some closed commands are valid but leave canonical bytes unchanged (for
	// example, applying the page setup already in force). They are not committed
	// mutations: preserve revision, dirty state, preview authority, and both
	// history branches exactly as they were.
	if bytes.Equal(canonical, e.bytes) {
		return e.Snapshot(), nil
	}
	// Reparse the candidate's canonical bytes before installation. Command
	// factories therefore cannot bypass the normal format validator, and the
	// persisted bytes, live template, and paint projection all describe the
	// same accepted document.
	installed, err := folio8.ParseTemplate(canonical)
	if err != nil {
		return Snapshot{}, err
	}
	projection, err = designer.CanvasWithTextPaint(installed, e.faces)
	if err != nil {
		return Snapshot{}, err
	}
	e.pushUndo(e.bytes)
	e.redo = nil
	e.install(installed, canonical, projection)
	return e.Snapshot(), nil
}

// Undo and Redo replay canonical engine bytes from this live wasm session.
// They never serialize history into .folio bytes or ask TypeScript to retain
// a mirror/inverse command. Revisions stay monotonic even when document bytes
// return to an earlier state, which keeps preview authority correlation sound.
func (e *Engine) Undo() (Snapshot, error) {
	if len(e.undo) == 0 {
		return e.Snapshot(), ErrNoUndo
	}
	prior := e.undo[len(e.undo)-1]
	e.undo = e.undo[:len(e.undo)-1]
	e.pushRedo(e.bytes)
	return e.restore(prior)
}

func (e *Engine) Redo() (Snapshot, error) {
	if len(e.redo) == 0 {
		return e.Snapshot(), ErrNoRedo
	}
	next := e.redo[len(e.redo)-1]
	e.redo = e.redo[:len(e.redo)-1]
	e.pushUndo(e.bytes)
	return e.restore(next)
}

func (e *Engine) restore(canonical []byte) (Snapshot, error) {
	tpl, err := folio8.ParseTemplate(canonical)
	if err != nil {
		return Snapshot{}, err
	}
	projection, err := designer.CanvasWithTextPaint(tpl, e.faces)
	if err != nil {
		return Snapshot{}, err
	}
	e.install(tpl, canonical, projection)
	return e.Snapshot(), nil
}

func (e *Engine) install(tpl *folio8.Template, canonical []byte, projection designer.CanvasProjection) {
	e.template = tpl
	e.bytes = append(e.bytes[:0], canonical...)
	e.canvas = &projection
	e.revision++
}

func (e *Engine) pushUndo(value []byte) { e.undo = appendBounded(e.undo, value) }
func (e *Engine) pushRedo(value []byte) { e.redo = appendBounded(e.redo, value) }
func appendBounded(history [][]byte, value []byte) [][]byte {
	copyValue := append([]byte(nil), value...)
	if len(history) == historyLimit {
		copy(history, history[1:])
		history[len(history)-1] = copyValue
		return history
	}
	return append(history, copyValue)
}
