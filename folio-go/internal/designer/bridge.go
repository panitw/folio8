// Package designer holds the folio8 designer's projection and command
// surface: the wire types the canvas paints from, their bounds, and the
// functions that project and edit a template.
//
// It is internal to folio-go on purpose. The public API of module root is
// render and validate; the designer is one in-module caller (internal/wasm),
// and nothing it needs is under semver.
//
// WHY FUNCTION VARIABLES AND NOT FUNCTIONS. The implementations read and
// write the unexported state of *folio8.Template and share the renderer's
// unexported helpers, so they stay in package folio8, unexported. This
// package cannot import folio8 (folio8 imports it for these types), so folio8
// assigns every variable below from its own init. A program reaches them by
// importing folio8 — which anything holding a *folio8.Template already does.
//
// Every tpl parameter is a *folio8.Template. Anything else is a programmer
// error and panics naming the expected type.
package designer

var (
	// Canvas projects a template's pages, bands and components without text
	// paint.
	Canvas func(tpl any) (CanvasProjection, error)
	// CanvasWithTextPaint projects the template with engine-shaped text paint
	// measured against fonts.
	CanvasWithTextPaint func(tpl any, fonts map[string][]byte) (CanvasProjection, error)
	// ApplyComponentCommand applies one component authoring command to tpl
	// in place and returns the new projection.
	ApplyComponentCommand func(tpl any, command []byte, fonts ...map[string][]byte) (CanvasProjection, error)
	// ApplyPageSetupCommand applies one page-setup command to tpl in place and
	// returns the new projection.
	ApplyPageSetupCommand func(tpl any, command []byte) (CanvasProjection, error)
	// PreviewComponentMove reports the translation a moveComponents command
	// would apply, without changing tpl.
	PreviewComponentMove func(tpl any, command []byte, fonts ...map[string][]byte) (ComponentMove, error)
	// TableColumns projects one table's columns.
	TableColumns func(tpl any, tableID string) (TableColumnsProjection, error)
	// PreviewIdentity returns opaque evidence for the inputs that can affect a
	// production preview.
	PreviewIdentity func(template, data, params []byte, fontSet map[string][]byte) string
	// AssetBytes returns one document asset's paintable bytes and media type.
	AssetBytes func(tpl any, key string) ([]byte, string, error)
	// StandInData returns a stand-in data document for tpl.
	StandInData func(tpl any) ([]byte, error)
	// CarriedCommands reports the commands one command applies, in order, and
	// whether the command is a unit that carries others. A unit reports its
	// members; every other command — and anything that does not decode as a
	// command object at all — reports itself, so one command and a unit of
	// commands read the same way.
	//
	// IT EXISTS SO THAT NOTHING OUTSIDE folio8 HAS TO KNOW A UNIT'S SHAPE.
	// internal/wasm's moveComponents staleness fence must see a move inside a
	// unit; it asks this question and then applies its own unchanged per-command
	// rule to each answer. A second parser for a unit's member list anywhere is
	// the defect this variable prevents: the fence and the applier would then be
	// free to disagree about what a unit contains.
	//
	// THE unit FLAG IS WHAT KEEPS A CALLER FROM SPEAKING OUT OF TURN. A member
	// a caller cannot decode is the unit door's to refuse, in the member's own
	// words; only a command that carries ITSELF and does not decode is the
	// caller's own malformed-input case. A unit whose member list cannot be read
	// reports no members and unit true, because the door states that refusal.
	CarriedCommands func(command []byte) (members [][]byte, unit bool)
)
