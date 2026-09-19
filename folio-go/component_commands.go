package folio8

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/panitw/folio8/folio-go/internal/designer"
	"github.com/panitw/folio8/folio-go/internal/expr"
	"github.com/panitw/folio8/folio-go/internal/fontset"
	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/layout"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// componentFailure is the one way the engine refuses a component command: a
// *designer.ComponentCommandError naming a paint-safe id and command field.
func componentFailure(id, path, message string) error {
	return designer.NewComponentCommandError(id, path, message)
}

// The two exported command doors both name a DOCUMENT rather than an element
// when they refuse ambiguous bytes, because a duplicate key means the id the
// command names is exactly the thing that cannot be trusted.
const (
	componentCommandPath = "command"
	pageSetupCommandPath = "page.setup"
)

// documentLocalePath and documentUTCOffsetPath are the DataPaths the two
// document-settings refusals carry. Each command names no element and no band —
// it writes ONE top-level document field — so ElementID stays empty and the
// path is the field's own name, which is also the key parse.go's own load error
// locates. Unlike bandHeightPath and fontChainPath they need no bound: nothing
// from the wire is interpolated into them, so neither can arrive at the host's
// 256-byte DataPath cut.
const (
	documentLocalePath    = "locale"
	documentUTCOffsetPath = "utcOffset"
)

// EACH DOOR'S REFUSAL CARRIES ITS OWN DIAGNOSTIC CODE, and the two are reached
// by different machinery at the host.
//
// wasm/cmd/engine/main.go's engineFailure matches *ComponentCommandError FIRST,
// before the page-setup fallback below it, and answers COMPONENT_INVALID. That
// is right for the component door and WRONG for page setup: the designer's only
// code-branching page-setup consumer keys on PAGE_SETUP_INVALID, so a page-setup
// refusal wearing COMPONENT_INVALID is a refusal that surface never sees.
//
// So the page-setup door returns a plain error whose message opens with the
// `folio8: page.` prefix the host's own fallback tests for. That path is not the
// unlocated ENGINE_REJECTED the "use componentFailure" rule exists to avoid — it
// sets DiagnosticCode PAGE_SETUP_INVALID, carries the message, and defaults
// DataPath to page.setup, with ElementID left empty exactly as required.
const pageSetupFailurePrefix = "folio8: " + pageSetupCommandPath

// maxCommandKeyScanDepth bounds the duplicate-key walk's recursion. It is not a
// rule about commands — the deepest one this vocabulary has is three objects —
// it is a bound on hostile bytes, because Decoder.Token() streams to ANY depth
// (measured: 20 000 nested arrays tokenise cleanly) while this walk is
// recursive.
//
// The number is encoding/json's own nesting limit, and that is the whole point:
// measured, json.Unmarshal refuses input nested deeper than 10 000 with
// "exceeded max depth", so a value this scan declines to walk is a value the
// caller's ordinary decode rejects on the very next line. The scan never has to
// be the thing that reports depth, and nothing decodable escapes it.
const maxCommandKeyScanDepth = 10000

// refuseDuplicateCommandKeys is the SOLE duplicate-key guard for both exported
// command doors, and it exists because every arity and version check on either
// door is duplicate-BLIND by construction. Both decode into
// map[string]json.RawMessage, which resolves a repeated key silently by
// last-wins BEFORE anything counts the map — so componentFields(raw, want) and
// len(raw) != 7 both count the deduplicated map, equalNumber(raw["version"],
// "1") reads the LAST version, and dispatch routes on the LAST kind.
//
// Executed at the baseline this replaces: a command that NAMED e1 in the page
// header MUTATED e5 in the page footer and returned a nil error. A version:0
// command was admitted by appending a second version:1. A deleteComponent was
// escalated into deleteFontChain because the two happen to have the same
// arity, and only the second handler's own field names stopped it — arity
// coincidence, not a check.
//
// encoding/json cannot report duplicates through any decode: by the time a map
// or a RawMessage is in hand, the duplicate is already gone. Token streaming is
// the only stdlib mechanism that sees them, and it is the idiom this module
// already uses (decoder.More at :519, internal/bind/value.go:161-183).
func refuseDuplicateCommandKeys(command []byte, door string) error {
	dec := json.NewDecoder(bytes.NewReader(command))
	dec.UseNumber()
	first, err := dec.Token()
	if err != nil {
		// Not this guard's refusal to make. Bytes that do not tokenise are
		// malformed, and the caller's own decode says so in its own words.
		return nil
	}
	at, key, found := scanForDuplicateKey(dec, first, "$", 0)
	if !found {
		return nil
	}
	message := fmt.Sprintf("the command declares the key %q twice at %s, so it names one thing and could change another", key, at)
	if door == pageSetupCommandPath {
		return fmt.Errorf("%s: %s", pageSetupFailurePrefix, message)
	}
	return componentFailure("", door, message)
}

// scanForDuplicateKey consumes the remainder of the value whose opening token
// is tok, and reports the path of the first object that declares a key twice.
// Arrays are walked too: an object nested inside one is not a special case
// anywhere else and must not be one here.
func scanForDuplicateKey(dec *json.Decoder, tok json.Token, path string, depth int) (string, string, bool) {
	delim, ok := tok.(json.Delim)
	if !ok {
		// A scalar is one token and it has already been read.
		return "", "", false
	}
	if depth > maxCommandKeyScanDepth {
		// DRAIN, NEVER JUST RETURN. Returning here without consuming the value
		// would leave the decoder positioned INSIDE it, so the parent object's
		// loop would read this value's nested keys as its own — reporting a
		// duplicate at a path that does not exist, or at no duplicate at all.
		// That would be a correctness bug in the very scanner this guard is.
		drainValue(dec, delim)
		return "", "", false
	}
	switch delim {
	case '{':
		// Lookup and insert only. lint/internal/rules/maprange.go bans ranging
		// a map in non-test Go, and nothing here needs to.
		seen := map[string]bool{}
		for dec.More() {
			keyToken, err := dec.Token()
			if err != nil {
				return "", "", false
			}
			key, ok := keyToken.(string)
			if !ok {
				return "", "", false
			}
			if seen[key] {
				return path, key, true
			}
			seen[key] = true
			valueToken, err := dec.Token()
			if err != nil {
				return "", "", false
			}
			if at, duplicate, found := scanForDuplicateKey(dec, valueToken, path+"."+key, depth+1); found {
				return at, duplicate, true
			}
		}
	case '[':
		for index := 0; dec.More(); index++ {
			valueToken, err := dec.Token()
			if err != nil {
				return "", "", false
			}
			if at, duplicate, found := scanForDuplicateKey(dec, valueToken, fmt.Sprintf("%s[%d]", path, index), depth+1); found {
				return at, duplicate, true
			}
		}
	}
	// The closing delimiter. Consuming it here is what leaves the decoder
	// positioned on the parent's next token.
	if _, err := dec.Token(); err != nil {
		return "", "", false
	}
	return "", "", false
}

// drainValue consumes the remainder of a composite value whose opening
// delimiter has already been read, leaving the decoder on the parent's next
// token. It is iterative on purpose: it exists to handle input too deeply
// nested to recurse over, so recursing over it would defeat itself.
func drainValue(dec *json.Decoder, opening json.Delim) {
	depth := 1
	if opening != '{' && opening != '[' {
		return
	}
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return
		}
		if delim, ok := tok.(json.Delim); ok {
			if delim == '{' || delim == '[' {
				depth++
			} else {
				depth--
			}
		}
	}
}

// applyComponentCommand applies Story 5.7's small, versioned authoring
// vocabulary. The command is intentionally decoded in Go: the browser sends
// opaque bytes and never receives the template or its canonical JSON shape.
// Optional fonts give window-constrained movement the same pagination as
// CanvasWithTextPaint. Other commands retain their existing behavior.
func applyComponentCommand(t *Template, command []byte, fonts ...FontSet) (designer.CanvasProjection, error) {
	if t == nil {
		return designer.CanvasProjection{}, errNilTemplate
	}
	if err := refuseDuplicateCommandKeys(command, componentCommandPath); err != nil {
		return designer.CanvasProjection{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(command))
	dec.UseNumber()
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return designer.CanvasProjection{}, fmt.Errorf("folio8: component command is malformed")
	}
	var surplus any
	if err := dec.Decode(&surplus); err != io.EOF {
		return designer.CanvasProjection{}, fmt.Errorf("folio8: component command is malformed")
	}
	if !equalNumber(raw["version"], "1") {
		return designer.CanvasProjection{}, fmt.Errorf("folio8: unknown component command")
	}
	var kind string
	if json.Unmarshal(raw["kind"], &kind) != nil {
		return designer.CanvasProjection{}, fmt.Errorf("folio8: unknown component command")
	}
	switch kind {
	case "createComponent":
		return createComponent(t, raw)
	case "dropComponent":
		return dropComponent(t, raw)
	case "moveComponent":
		return moveComponent(t, raw)
	case "moveComponents":
		return moveComponents(t, raw, fonts...)
	case "resizeComponent":
		return resizeComponent(t, raw)
	case "setComponentBounds":
		return setComponentBounds(t, raw)
	case "deleteComponent":
		return deleteComponent(t, raw)
	case "duplicateComponent":
		return duplicateComponent(t, raw)
	case "deleteComponents":
		return deleteComponents(t, raw)
	case "duplicateComponents":
		return duplicateComponents(t, raw)
	case "updateComponentProperties":
		return updateComponentProperties(t, raw)
	case "setComponentAsset":
		return setComponentAsset(t, raw)
	case "bindComponentScalar":
		return bindComponentScalar(t, raw)
	case "addTableColumn":
		return applyTableColumnCommand(t, raw, addTableColumn)
	case "removeTableColumn":
		return applyTableColumnCommand(t, raw, removeTableColumn)
	case "moveTableColumn":
		return applyTableColumnCommand(t, raw, moveTableColumn)
	case "updateTableColumn":
		return applyTableColumnCommand(t, raw, updateTableColumn)
	case "setTableWidth":
		return applyTableColumnCommand(t, raw, setTableWidth)
	case "configureTableBinding":
		return applyTableColumnCommand(t, raw, configureTableBinding)
	case "bindTableCollection":
		return applyTableColumnCommand(t, raw, bindTableCollection)
	case "updateTableColumnBinding":
		return applyTableColumnCommand(t, raw, updateTableColumnBinding)
	case "updateTableColumnExpression":
		return applyTableColumnCommand(t, raw, updateTableColumnExpression)
	case "updateTableColumnFooter":
		return applyTableColumnCommand(t, raw, updateTableColumnFooter)
	case "addFontChain":
		return applyFontChainCommand(t, raw, addFontChain)
	case "renameFontChain":
		return applyFontChainCommand(t, raw, renameFontChain)
	case "deleteFontChain":
		return applyFontChainCommand(t, raw, deleteFontChain)
	case "addFontChainEntry":
		return applyFontChainCommand(t, raw, addFontChainEntry)
	case "moveFontChainEntry":
		return applyFontChainCommand(t, raw, moveFontChainEntry)
	case "removeFontChainEntry":
		return applyFontChainCommand(t, raw, removeFontChainEntry)
	case "embedFontFamily":
		return applyFontChainCommand(t, raw, embedFontFamily)
	case "setBandHeight":
		return setBandHeight(t, raw)
	case "setSectionBreak":
		return setSectionBreak(t, raw)
	case "removeSectionBreak":
		return removeSectionBreak(t, raw)
	case "setSectionBreakAnchor":
		return setSectionBreakAnchor(t, raw)
	case "addPage":
		return addPage(t, raw)
	case "deletePage":
		return deletePage(t, raw)
	case "setPageBreak":
		return setPageBreak(t, raw)
	case "setDocumentLocale":
		return setDocumentLocale(t, raw)
	case "setDocumentUTCOffset":
		return setDocumentUTCOffset(t, raw)
	case "setTableHeaderHeight":
		return applyTableColumnCommand(t, raw, setTableHeaderHeight)
	case "setTableAltRowBackground":
		return applyTableColumnCommand(t, raw, setTableAltRowBackground)
	case "updateTableHeaderStyle":
		return applyTableColumnCommand(t, raw, updateTableHeaderStyle)
	case "setTableMinHeight":
		return applyTableColumnCommand(t, raw, setTableMinHeight)
	case "updateTableRules":
		return applyTableColumnCommand(t, raw, updateTableRules)
	default:
		return designer.CanvasProjection{}, fmt.Errorf("folio8: unknown component command")
	}
}

// applyTableColumnCommand keeps the public command seam just as atomic as
// wasm.Engine.Apply. The individual handlers may mutate their candidate while
// checking geometry, but the caller's template is installed only after that
// candidate serializes, reparses, and projects successfully.
func applyTableColumnCommand(t *Template, raw map[string]json.RawMessage, apply func(*Template, map[string]json.RawMessage) (designer.CanvasProjection, error)) (designer.CanvasProjection, error) {
	before, err := SerializeTemplate(t)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	working, err := ParseTemplate(before)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	if _, err := apply(working, raw); err != nil {
		return designer.CanvasProjection{}, err
	}
	canonical, err := SerializeTemplate(working)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	installed, err := ParseTemplate(canonical)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	projection, err := canvas(installed)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	t.doc, t.derivedFooters = installed.doc, installed.derivedFooters
	return projection, nil
}

// The table commands are a deliberately closed authoring vocabulary. Sample
// input never enters these commands: it only helps the UI discover candidates.
func addTableColumn(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	if err := componentFields(raw, 4); err != nil {
		return designer.CanvasProjection{}, err
	}
	id, err := commandString(raw, "id")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure("", "table.id", err.Error())
	}
	index, err := commandInt(raw, "index")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "column.index", err.Error())
	}
	_, band, _, element, err := findComponent(t, id)
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "table.id", "table was not found")
	}
	if element.Type != template.ElementTable || !element.Table.Set || element.Table.Null {
		return designer.CanvasProjection{}, componentFailure(id, "table.id", "component is not a table")
	}
	columns := element.Table.Value.Columns
	if len(columns) >= maxTableColumns {
		return designer.CanvasProjection{}, componentFailure(id, "column.index", "table has too many columns")
	}
	if index < 0 || index > len(columns) {
		return designer.CanvasProjection{}, componentFailure(id, "column.index", "column index is out of range")
	}
	if t.doc.NextID <= 0 || t.doc.NextID == 1<<63-1 {
		return designer.CanvasProjection{}, componentFailure(id, "column.id", "nextId cannot allocate another column")
	}
	// Keep the normal width when it fits. Otherwise split one existing column
	// in exact millipoints; the candidate clone makes the resize and insertion
	// atomic, including a later containment or canonical-validation refusal.
	width, _ := projectedSize(*element)
	newWidth := geom.Length(72000)
	if !element.Width.Set && newWidth > geom.Length(band.Width)-element.X-width {
		widest := -1
		for i, existing := range columns {
			if existing.Width > 1 && (widest < 0 || existing.Width > columns[widest].Width) {
				widest = i
			}
		}
		if widest < 0 {
			return designer.CanvasProjection{}, componentFailure(id, "column.width", "no column can be split into two positive widths")
		}
		newWidth = columns[widest].Width / 2
		// The existing column keeps an odd millipoint; the first widest wins ties.
		columns[widest].Width -= newWidth
	}
	column := template.Column{ID: template.AllocateElementID(t.doc), Label: fmt.Sprintf("Column %d", len(columns)+1), Width: newWidth}
	if element.Width.Set {
		column.Width = 0
		column.Proportion = template.Presence[int64]{Set: true, Value: template.ProportionUnit}
	}
	element.Table.Value.Columns = append(columns, template.Column{})
	copy(element.Table.Value.Columns[index+1:], element.Table.Value.Columns[index:])
	element.Table.Value.Columns[index] = column
	if _, err := template.TableColumnWidths(*element); err != nil {
		return designer.CanvasProjection{}, wrapTableWidthError(err)
	}
	width, height := projectedSize(*element)
	if err := containComponent(band, element.X, element.Y, width, height); err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "column.width", err.Error())
	}
	t.doc.NextID++
	return canvas(t)
}

func removeTableColumn(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	if err := componentFields(raw, 4); err != nil {
		return designer.CanvasProjection{}, err
	}
	id, err := commandString(raw, "id")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure("", "table.id", err.Error())
	}
	columnID, err := commandString(raw, "columnId")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "column.id", err.Error())
	}
	_, _, _, element, err := findComponent(t, id)
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "table.id", "table was not found")
	}
	if element.Type != template.ElementTable || !element.Table.Set || element.Table.Null {
		return designer.CanvasProjection{}, componentFailure(id, "table.id", "component is not a table")
	}
	index := tableColumnIndex(element, columnID)
	if index < 0 {
		return designer.CanvasProjection{}, componentFailure(id, "column.id", "column was not found")
	}
	columns := element.Table.Value.Columns
	copy(columns[index:], columns[index+1:])
	element.Table.Value.Columns = columns[:len(columns)-1]
	return canvas(t)
}

func moveTableColumn(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	if err := componentFields(raw, 5); err != nil {
		return designer.CanvasProjection{}, err
	}
	id, err := commandString(raw, "id")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure("", "table.id", err.Error())
	}
	columnID, err := commandString(raw, "columnId")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "column.id", err.Error())
	}
	toIndex, err := commandInt(raw, "toIndex")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "column.toIndex", err.Error())
	}
	_, _, _, element, err := findComponent(t, id)
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "table.id", "table was not found")
	}
	if element.Type != template.ElementTable || !element.Table.Set || element.Table.Null {
		return designer.CanvasProjection{}, componentFailure(id, "table.id", "component is not a table")
	}
	fromIndex := tableColumnIndex(element, columnID)
	if fromIndex < 0 {
		return designer.CanvasProjection{}, componentFailure(id, "column.id", "column was not found")
	}
	columns := element.Table.Value.Columns
	if toIndex < 0 || toIndex >= len(columns) {
		return designer.CanvasProjection{}, componentFailure(id, "column.toIndex", "column index is out of range")
	}
	if fromIndex == toIndex {
		return canvas(t)
	}
	column := columns[fromIndex]
	if fromIndex < toIndex {
		copy(columns[fromIndex:toIndex], columns[fromIndex+1:toIndex+1])
	} else {
		copy(columns[toIndex+1:fromIndex+1], columns[toIndex:fromIndex])
	}
	columns[toIndex] = column
	return canvas(t)
}

func updateTableColumn(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	if err := componentFields(raw, 6); err != nil {
		return designer.CanvasProjection{}, err
	}
	id, err := commandString(raw, "id")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure("", "table.id", err.Error())
	}
	columnID, err := commandString(raw, "columnId")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "column.id", err.Error())
	}
	field, err := commandString(raw, "field")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "column.field", err.Error())
	}
	value, ok := raw["value"]
	if !ok {
		return designer.CanvasProjection{}, componentFailure(id, "column.value", "column value is required")
	}
	_, band, _, element, err := findComponent(t, id)
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "table.id", "table was not found")
	}
	if element.Type != template.ElementTable || !element.Table.Set || element.Table.Null {
		return designer.CanvasProjection{}, componentFailure(id, "table.id", "component is not a table")
	}
	index := tableColumnIndex(element, columnID)
	if index < 0 {
		return designer.CanvasProjection{}, componentFailure(id, "column.id", "column was not found")
	}
	column := &element.Table.Value.Columns[index]
	switch field {
	case "header":
		label, err := commandString(map[string]json.RawMessage{"value": value}, "value")
		// ⚠ CHARACTERS, NOT BYTES (SPEC-table-rules §4). The bound was
		// `len(label)`, which counts UTF-8 BYTES: 256 of them is about 85
		// Thai characters, and a bilingual two-line heading is exactly the
		// label an author now writes here. The browser's own bound
		// (engine-protocol.ts, `column.header.length <= 256`) has always
		// counted units of text rather than bytes, so this also makes the
		// two doors agree — a label the panel accepted and the engine
		// refused was a refusal with no field the author could see.
		if err != nil || utf8.RuneCountInString(label) > 256 {
			return designer.CanvasProjection{}, componentFailure(id, "column.header", "header must be at most 256 characters")
		}
		column.Label = label
	case "width":
		if element.Width.Set {
			return designer.CanvasProjection{}, componentFailure(id, "column.width", "edit the proportion in proportional sizing")
		}
		width, err := authoredTableLength(value, "width")
		if err != nil || width <= 0 {
			return designer.CanvasProjection{}, componentFailure(id, "column.width", "width must be a positive length")
		}
		column.Width = width
	case "proportion":
		if !element.Width.Set {
			return designer.CanvasProjection{}, componentFailure(id, "column.proportion", "this table uses point widths")
		}
		literal := string(value)
		if len(value) > 0 && value[0] == '"' {
			if err := json.Unmarshal(value, &literal); err != nil {
				return designer.CanvasProjection{}, componentFailure(id, "column.proportion", "proportion must be a decimal")
			}
		}
		proportion, err := template.DecodeProportion(literal)
		if err != nil {
			return designer.CanvasProjection{}, componentFailure(columnID, "column.proportion", err.Error())
		}
		column.Proportion = template.Presence[int64]{Set: true, Value: proportion}
	case "align":
		align, err := commandString(map[string]json.RawMessage{"value": value}, "value")
		if err != nil || (align != "left" && align != "center" && align != "right") {
			return designer.CanvasProjection{}, componentFailure(id, "column.align", "alignment must be left, center, or right")
		}
		column.Align = template.Presence[string]{Set: true, Value: align}
	case "headerAlign":
		// SET-ONLY, by the owner's ruling (2026-09-13): the header control is
		// always explicit, so there is no clear and no "follow cell" value. The
		// closed set is the loader's own (template.IsColumnHeaderAlign), so the
		// command door and the file door cannot admit different values.
		headerAlign, err := commandString(map[string]json.RawMessage{"value": value}, "value")
		if err != nil || !template.IsColumnHeaderAlign(headerAlign) {
			return designer.CanvasProjection{}, componentFailure(id, "column.headerAlign", "header alignment must be left, center, or right")
		}
		column.HeaderAlign = template.Presence[string]{Set: true, Value: headerAlign}
	default:
		return designer.CanvasProjection{}, componentFailure(id, "column.field", "column field is not editable")
	}
	if _, err := template.TableColumnWidths(*element); err != nil {
		return designer.CanvasProjection{}, wrapTableWidthError(err)
	}
	width, height := projectedSize(*element)
	if err := containComponent(band, element.X, element.Y, width, height); err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "column.width", err.Error())
	}
	return canvas(t)
}

// Table controls send untouched text; Go owns both decimal validation and
// geometry. Existing number-valued point-width callers remain supported.
func authoredTableLength(raw json.RawMessage, field string) (geom.Length, error) {
	if len(raw) > 0 && raw[0] == '"' {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return 0, err
		}
		raw = json.RawMessage(text)
	}
	return propertyLength(raw, field)
}

func setTableWidth(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	if err := componentFields(raw, 4); err != nil {
		return designer.CanvasProjection{}, err
	}
	id, err := commandString(raw, "id")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure("", "table.id", err.Error())
	}
	_, band, _, element, err := findComponent(t, id)
	if err != nil || element.Type != template.ElementTable || !element.Table.Set || element.Table.Null {
		return designer.CanvasProjection{}, componentFailure(id, "table.id", "table was not found")
	}
	if !element.Width.Set {
		return designer.CanvasProjection{}, componentFailure(id, "table.width", "total width requires proportional columns")
	}
	width, err := authoredTableLength(raw["value"], "width")
	if err != nil || width <= 0 {
		return designer.CanvasProjection{}, componentFailure(id, "table.width", "total width must be a positive decimal with at most three decimal places")
	}
	element.Width.Value = width
	if _, err := template.TableColumnWidths(*element); err != nil {
		return designer.CanvasProjection{}, wrapTableWidthError(err)
	}
	_, height := projectedSize(*element)
	if err := containComponent(band, element.X, element.Y, width, height); err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "table.width", err.Error())
	}
	return canvas(t)
}

var rootCollectionPath = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*\[\]$`)
var boundedIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var rootValuePath = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*$`)

// bindTableCollection accepts decoded sample keys without changing aliases or
// column expressions. Explicit footer sources follow the collection while
// retaining their row-relative fields.
func bindTableCollection(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	if err := componentFields(raw, 4); err != nil {
		return designer.CanvasProjection{}, err
	}
	id, err := commandString(raw, "id")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure("", "table.id", err.Error())
	}
	var segments []string
	if json.Unmarshal(raw["segments"], &segments) != nil || len(segments) == 0 {
		return designer.CanvasProjection{}, componentFailure(id, "table.collection", "collection segments must be a non-empty string array")
	}
	for _, segment := range segments {
		// Check each decoded key before joining, so a key containing a dot can
		// never be silently reinterpreted as multiple object keys.
		if !boundedIdentifier.MatchString(segment) {
			return designer.CanvasProjection{}, componentFailure(id, "table.collection", "collection segments must be identifiers")
		}
	}
	collection := strings.Join(segments, ".") + "[]"
	if !validRootCollection(collection) {
		return designer.CanvasProjection{}, componentFailure(id, "table.collection", "collection must be a bounded root collection path ending in []")
	}
	_, _, _, element, err := findComponent(t, id)
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "table.id", "table was not found")
	}
	if element.Type != template.ElementTable || !element.Table.Set || element.Table.Null {
		return designer.CanvasProjection{}, componentFailure(id, "table.id", "component is not a table")
	}
	if err := setTableCollection(element, collection); err != nil {
		return designer.CanvasProjection{}, err
	}
	return canvas(t)
}

func validRootCollection(collection string) bool {
	return len(collection) <= maxCanvasBindingString && rootCollectionPath.MatchString(collection) && !strings.HasPrefix(collection, "params.") && collection != "params[]"
}

// Both collection-editing commands preserve explicit footer source fields.
// Keep the resulting source within the TableColumns projection's bound so
// an accepted edit cannot make Configure columns unavailable.
func setTableCollection(element *template.Element, collection string) error {
	table := &element.Table.Value
	oldPrefix := strings.TrimSuffix(table.Bind, "[]") + "."
	newPrefix := strings.TrimSuffix(collection, "[]") + "."
	for index := range table.Columns {
		source := &table.Columns[index].FooterOf
		if !source.Set || source.Null {
			continue
		}
		field, ok := strings.CutPrefix(source.Value, oldPrefix)
		if !ok || len(newPrefix)+len(field) > maxCanvasBindingString {
			return componentFailure(string(element.ID), "column.footerOf", "footer source must remain within the table collection and the 256-byte editor bound")
		}
		source.Value = newPrefix + field
	}
	table.Bind = collection
	return nil
}

// configureTableBinding changes the two document-owned row-scope settings as
// one candidate. An empty alias deliberately means the schema's absent `as`
// form; render resolution supplies the established default alias, `row`.
func configureTableBinding(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	if err := componentFields(raw, 5); err != nil {
		return designer.CanvasProjection{}, err
	}
	id, err := commandString(raw, "id")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure("", "table.id", err.Error())
	}
	collection, err := commandString(raw, "collection")
	if err != nil || !validRootCollection(collection) {
		return designer.CanvasProjection{}, componentFailure(id, "table.collection", "collection must be a bounded root collection path ending in []")
	}
	aliasRaw, ok := raw["alias"]
	if !ok {
		return designer.CanvasProjection{}, componentFailure(id, "table.alias", "alias is required")
	}
	var alias string
	if json.Unmarshal(aliasRaw, &alias) != nil || len(alias) > 64 || (alias != "" && (!boundedIdentifier.MatchString(alias) || reservedRowAlias(alias))) {
		return designer.CanvasProjection{}, componentFailure(id, "table.alias", "alias must be a bounded identifier or empty for row")
	}
	_, _, _, element, err := findComponent(t, id)
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "table.id", "table was not found")
	}
	if element.Type != template.ElementTable || !element.Table.Set || element.Table.Null {
		return designer.CanvasProjection{}, componentFailure(id, "table.id", "component is not a table")
	}
	oldAlias := resolvedTableAlias(element.Table.Value.As)
	newAlias := alias
	if newAlias == "" {
		newAlias = "row"
	}
	if oldAlias != newAlias {
		for i := range element.Table.Value.Columns {
			next, migrated, used, migrationErr := expr.RewriteRowBinding(element.Table.Value.Columns[i].Bind, oldAlias, newAlias)
			if migrationErr != nil || (used && !migrated) {
				return designer.CanvasProjection{}, componentFailure(id, "table.alias", "alias change cannot migrate a row-scoped column binding")
			}
			if migrated {
				if len(next) > maxCanvasBindingString {
					return designer.CanvasProjection{}, componentFailure(id, "table.alias", "alias change would exceed the table editor binding text limit")
				}
				element.Table.Value.Columns[i].Bind = next
			}
		}
	}
	if err := setTableCollection(element, collection); err != nil {
		return designer.CanvasProjection{}, err
	}
	if alias == "" {
		element.Table.Value.As = template.Presence[string]{}
	} else {
		element.Table.Value.As = template.Presence[string]{Set: true, Value: alias}
	}
	return canvas(t)
}

// updateTableColumnBinding accepts a single row-relative field path or an
// empty field to clear. Go owns the actual expression spelling.
func updateTableColumnBinding(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	if err := componentFields(raw, 5); err != nil {
		return designer.CanvasProjection{}, err
	}
	id, err := commandString(raw, "id")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure("", "table.id", err.Error())
	}
	columnID, err := commandString(raw, "columnId")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "column.id", err.Error())
	}
	field, ok := optionalCommandString(raw, "field", 192)
	if !ok || bytes.Equal(bytes.TrimSpace(raw["field"]), []byte("null")) || (field != "" && !rootValuePath.MatchString(field)) {
		return designer.CanvasProjection{}, componentFailure(id, "column.bind", "field must be a bounded row field path")
	}
	_, _, _, element, err := findComponent(t, id)
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "table.id", "table was not found")
	}
	if element.Type != template.ElementTable || !element.Table.Set || element.Table.Null {
		return designer.CanvasProjection{}, componentFailure(id, "table.id", "component is not a table")
	}
	index := tableColumnIndex(element, columnID)
	if index < 0 {
		return designer.CanvasProjection{}, componentFailure(id, "column.id", "column was not found")
	}
	if field == "" {
		element.Table.Value.Columns[index].Bind = ""
		return canvas(t)
	}
	alias := "row"
	if element.Table.Value.As.Set && !element.Table.Value.As.Null {
		alias = element.Table.Value.As.Value
	}
	binding := "{{" + alias + "." + field + "}}"
	if len(binding) > maxCanvasBindingString {
		return designer.CanvasProjection{}, componentFailure(id, "column.bind", "binding with the current row alias exceeds the table editor text limit")
	}
	element.Table.Value.Columns[index].Bind = binding
	return canvas(t)
}

// updateTableColumnExpression accepts the complete authored text unchanged.
// applyTableColumnCommand reparses the candidate with canonical text-expression
// and footer validation before installing any document changes.
func updateTableColumnExpression(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	if err := componentFields(raw, 5); err != nil {
		return designer.CanvasProjection{}, err
	}
	id, err := commandString(raw, "id")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure("", "table.id", err.Error())
	}
	columnID, err := commandString(raw, "columnId")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "column.id", err.Error())
	}
	binding, ok := optionalCommandString(raw, "binding", maxCanvasBindingString)
	if !ok || bytes.Equal(bytes.TrimSpace(raw["binding"]), []byte("null")) {
		return designer.CanvasProjection{}, componentFailure(id, "column.bind", "binding must be an explicit string of at most 256 bytes")
	}
	_, _, _, element, err := findComponent(t, id)
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "table.id", "table was not found")
	}
	if element.Type != template.ElementTable || !element.Table.Set || element.Table.Null {
		return designer.CanvasProjection{}, componentFailure(id, "table.id", "component is not a table")
	}
	index := tableColumnIndex(element, columnID)
	if index < 0 {
		return designer.CanvasProjection{}, componentFailure(id, "column.id", "column was not found")
	}
	element.Table.Value.Columns[index].Bind = binding
	return canvas(t)
}

func reservedRowAlias(alias string) bool {
	return alias == "params" || alias == "page" || alias == "pages"
}

func resolvedTableAlias(value template.Presence[string]) string {
	if value.Set && !value.Null && value.Value != "" {
		return value.Value
	}
	return "row"
}

// updateTableColumnFooter is intentionally a complete footer configuration,
// not three independent mutations. Empty companion strings mean absent schema
// fields, making an accepted command one revision/history step.
func updateTableColumnFooter(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	if err := componentFields(raw, 7); err != nil {
		return designer.CanvasProjection{}, err
	}
	id, err := commandString(raw, "id")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure("", "table.id", err.Error())
	}
	columnID, err := commandString(raw, "columnId")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "column.id", err.Error())
	}
	footer, ok := optionalCommandString(raw, "footer", 16)
	if !ok || (footer != "" && footer != "sum" && footer != "avg" && footer != "count") {
		return designer.CanvasProjection{}, componentFailure(id, "column.footer", "footer must be sum, avg, count, or empty")
	}
	footerOf, ok := optionalCommandString(raw, "footerOf", 256)
	if !ok || (footerOf != "" && !rootValuePath.MatchString(footerOf)) {
		return designer.CanvasProjection{}, componentFailure(id, "column.footerOf", "footerOf must be a bounded root data path")
	}
	footerFormat, ok := optionalCommandString(raw, "footerFormat", 256)
	if !ok {
		return designer.CanvasProjection{}, componentFailure(id, "column.footerFormat", "footerFormat must be a bounded string")
	}
	if footer == "" && (footerOf != "" || footerFormat != "") {
		return designer.CanvasProjection{}, componentFailure(id, "column.footer", "footer companions require a footer")
	}
	if footer == "count" && footerOf != "" {
		return designer.CanvasProjection{}, componentFailure(id, "column.footerOf", "count uses the table collection and forbids footerOf")
	}
	_, _, _, element, err := findComponent(t, id)
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "table.id", "table was not found")
	}
	if element.Type != template.ElementTable || !element.Table.Set || element.Table.Null {
		return designer.CanvasProjection{}, componentFailure(id, "table.id", "component is not a table")
	}
	index := tableColumnIndex(element, columnID)
	if index < 0 {
		return designer.CanvasProjection{}, componentFailure(id, "column.id", "column was not found")
	}
	collection := strings.TrimSuffix(element.Table.Value.Bind, "[]")
	if footerOf != "" && !strings.HasPrefix(footerOf, collection+".") {
		return designer.CanvasProjection{}, componentFailure(id, "column.footerOf", "footerOf must stay within the table collection")
	}
	column := &element.Table.Value.Columns[index]
	column.Footer, column.FooterOf, column.FooterFormat = template.Presence[string]{}, template.Presence[string]{}, template.Presence[string]{}
	if footer != "" {
		column.Footer = template.Presence[string]{Set: true, Value: footer}
	}
	if footerOf != "" {
		column.FooterOf = template.Presence[string]{Set: true, Value: footerOf}
	}
	if footerFormat != "" {
		column.FooterFormat = template.Presence[string]{Set: true, Value: footerFormat}
	}
	return canvas(t)
}

func optionalCommandString(raw map[string]json.RawMessage, name string, max int) (string, bool) {
	v, ok := raw[name]
	if !ok {
		return "", false
	}
	var out string
	if json.Unmarshal(v, &out) != nil || len(out) > max {
		return "", false
	}
	return out, true
}

func tableColumnIndex(element *template.Element, columnID string) int {
	for index, column := range element.Table.Value.Columns {
		if string(column.ID) == columnID {
			return index
		}
	}
	return -1
}

func commandInt(raw map[string]json.RawMessage, name string) (int, error) {
	value, ok := raw[name]
	if !ok {
		return 0, fmt.Errorf("%s is required", name)
	}
	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err != nil || decoder.More() {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	integer, err := number.Int64()
	if err != nil || int64(int(integer)) != integer {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	return int(integer), nil
}

// bindComponentScalar is the sole Story 6.2 mutation for a picked root data
// path. The caller transports JSON object-key segments, not an expression or a
// browser-side validity judgment. This command owns the conversion to folio8's
// established expression grammar and rejects every non-root/reserved form
// before touching the target element.
func bindComponentScalar(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	if err := componentFields(raw, 4); err != nil {
		return designer.CanvasProjection{}, err
	}
	id, err := commandString(raw, "id")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure("", "component.id", err.Error())
	}
	segmentsRaw, ok := raw["segments"]
	if !ok {
		return designer.CanvasProjection{}, componentFailure(id, "binding.segments", "binding segments are required")
	}
	var segments []string
	if json.Unmarshal(segmentsRaw, &segments) != nil || len(segments) == 0 || len(segments) > 32 {
		return designer.CanvasProjection{}, componentFailure(id, "binding.segments", "binding segments must be a non-empty bounded string array")
	}
	for _, segment := range segments {
		if segment == "" || len(segment) > 64 {
			return designer.CanvasProjection{}, componentFailure(id, "binding.segments", "binding segments must be bounded non-empty strings")
		}
	}
	if segments[0] == "params" {
		return designer.CanvasProjection{}, componentFailure(id, "binding.segments", "params is not a root data binding")
	}
	path := strings.Join(segments, ".")
	if len(path) > maxCanvasBindingString {
		return designer.CanvasProjection{}, componentFailure(id, "binding.segments", "binding path exceeds the projection bound")
	}
	parsed, err := expr.Parse(path)
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "binding.segments", "binding path is not a valid folio8 expression")
	}
	pathExpr, ok := parsed.(*expr.PathExpr)
	// Joining is only an intermediate representation for folio8's established
	// identifier grammar. It must never reinterpret a decoded JSON key such as
	// "a.b" as two keys. Keys that folio8 cannot represent are rejected before
	// mutation, rather than silently binding a different path.
	if !ok || !sameSegments(pathExpr.Segments, segments) || expr.IsReserved(path) {
		return designer.CanvasProjection{}, componentFailure(id, "binding.segments", "binding path must be a non-reserved root data path")
	}
	if err := expr.Check(parsed); err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "binding.segments", "binding path is not a valid folio8 expression")
	}
	_, _, _, element, err := findComponent(t, id)
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "component.id", "component was not found")
	}
	if element.Type != template.ElementText && element.Type != template.ElementBarcode && element.Type != template.ElementQRCode {
		return designer.CanvasProjection{}, componentFailure(id, "component.id", "only text, barcode and qrcode components can receive a scalar binding")
	}
	// The generated expression is canonical and then independently reparsed by
	// wasm.Engine before installation. No sample bytes or local tree metadata
	// enter the template.
	element.Value = template.Presence[string]{Set: true, Value: "{{" + path + "}}"}
	return canvas(t)
}

func sameSegments(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

// updateComponentProperties is deliberately a small closed mutation language.
// It applies the supplied changes to every named component as one candidate;
// the engine's serialize/reparse transaction makes the update atomic.
func updateComponentProperties(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	before, err := SerializeTemplate(t)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	working, err := ParseTemplate(before)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	projection, err := updateComponentPropertiesInPlace(working, raw)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	// Keep the public helper transactional too. wasm.Apply uses a fresh clone,
	// but direct callers must receive the same no-partial-mutation guarantee.
	canonical, err := SerializeTemplate(working)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	installed, err := ParseTemplate(canonical)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	t.doc, t.derivedFooters = installed.doc, installed.derivedFooters
	return projection, nil
}

// engineProtocolMaxPayloadBytes mirrors MAX_ENGINE_PAYLOAD_BYTES
// (folio-designer/src/engine-protocol.ts) — the one number the transport
// enforces on every worker request payload, asset commands included.
const engineProtocolMaxPayloadBytes = 8 * 1024 * 1024

// maxComponentAssetPayloadOverheadBytes reserves room, inside the envelope
// above, for setComponentAssetCommand's OWN JSON skeleton around its
// base64 "data" field — the "kind"/"version"/"id"/"mediaType" keys and
// values (component-asset-command.ts). id is bounded by
// MAX_ENGINE_ELEMENT_ID_LENGTH (128, engine-protocol.ts) and mediaType is
// a handful of ASCII bytes in practice ("image/png", "image/jpeg"); even a
// pathological worst case with every id/mediaType byte JSON-escaped to
// \uXXXX (6 bytes each) stays under 2 KiB. 4 KiB leaves comfortable
// headroom without materially shrinking the budget below.
const maxComponentAssetPayloadOverheadBytes = 4 * 1024

// maxComponentAssetBytes is D-5.13.4's host-memory bound, DERIVED from the
// protocol envelope rather than reused verbatim (Finding 6, review of
// 2026-08-29). The command travels as JSON containing BASE64 — a 4/3
// expansion — so applying the raw 8 MiB envelope ceiling directly to the
// DECODED byte count (as this constant did before the fix) let a file
// between roughly 6 and 8 MiB pass Go's own check while the base64-inflated
// envelope had already rejected it at the TRANSPORT, before the command
// diagnostic AC2 requires could ever be produced — the protocol threshold
// and the author-facing diagnostic disagreed, which D-5.13.4 explicitly
// forbids ("the two must not disagree about the threshold"). This is
// instead the largest DECODED size whose base64-encoded command payload,
// plus the skeleton overhead above, is still guaranteed to fit inside
// engineProtocolMaxPayloadBytes — so a file Go is willing to accept can
// always actually arrive. It remains, as before, a memory judgement rather
// than an arithmetic proof like maxImagePixelDimension's (int64 overflow
// in geom.ScaleRound) — only its GROUND changed, not its honesty about
// what kind of number it is.
const maxComponentAssetBytes = (engineProtocolMaxPayloadBytes - maxComponentAssetPayloadOverheadBytes) * 3 / 4

// setComponentAsset is AC1/AC4's asset-authoring command (D-5.13.1): a
// closed, two-value payload (raw bytes plus declared media type) that
// propertyChange's {op,value} grammar cannot express, and AC4's rule that an
// image element is never legally asset-less means clear/null must stay
// inexpressible for it. It is therefore its own top-level command kind, not
// a key threaded through applyPropertyChanges/propertyPath/allowed.
//
// Go alone hashes the decoded bytes, recognises the media type (reusing
// image.go's DecodeImageForRender rather than a second capability check),
// inserts the asset only if its key is absent, repoints the target element,
// and collects the previous asset key it just orphaned — scoped to that one
// key, never a document-wide sweep (D-5.13.3). The whole thing runs inside
// one serialize/reparse/project transaction, matching every other component
// command's no-partial-mutation guarantee.
func setComponentAsset(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	before, err := SerializeTemplate(t)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	working, err := ParseTemplate(before)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	projection, err := setComponentAssetInPlace(working, raw)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	canonical, err := SerializeTemplate(working)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	installed, err := ParseTemplate(canonical)
	if err != nil {
		return designer.CanvasProjection{}, componentFailure("", "component.asset", "component asset did not pass format validation")
	}
	t.doc, t.derivedFooters = installed.doc, installed.derivedFooters
	return projection, nil
}

func setComponentAssetInPlace(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	if err := componentFields(raw, 5); err != nil {
		return designer.CanvasProjection{}, err
	}
	id, err := commandString(raw, "id")
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	_, _, _, element, err := findComponent(t, id)
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "component.id", "component was not found")
	}
	if element.Type != template.ElementImage {
		return designer.CanvasProjection{}, componentFailure(id, "component.id", "only an image component can receive an asset")
	}
	mediaType, err := commandString(raw, "mediaType")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "component.mediaType", err.Error())
	}
	dataRaw, ok := raw["data"]
	if !ok {
		return designer.CanvasProjection{}, componentFailure(id, "component.data", "asset data is required")
	}
	var dataB64 string
	if json.Unmarshal(dataRaw, &dataB64) != nil || dataB64 == "" {
		return designer.CanvasProjection{}, componentFailure(id, "component.data", "asset data must be a non-empty base64 string")
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(dataB64)
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "component.data", "asset data must be valid base64")
	}
	if len(decoded) == 0 {
		return designer.CanvasProjection{}, componentFailure(id, "component.data", "asset data cannot be empty")
	}
	if len(decoded) > maxComponentAssetBytes {
		return designer.CanvasProjection{}, componentFailure(id, "component.data", fmt.Sprintf("asset exceeds the %d-byte supported size", maxComponentAssetBytes))
	}
	digest := sha256.Sum256(decoded)
	key := fmt.Sprintf("%x", digest)
	// AC1: media-type recognition and decode validation happen at the
	// COMMAND, never relying on decodeAssets (parse.go) as the catcher — a
	// file this library version cannot decode is refused here, before
	// anything is written to t.doc.Assets.
	if _, err := template.DecodeImageForRender(mediaType, decoded, key, id); err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "component.mediaType", err.Error())
	}
	previousKey := ""
	if element.Asset.Set && !element.Asset.Null {
		previousKey = element.Asset.Value
	}
	if t.doc.Assets == nil {
		t.doc.Assets = map[string]template.Asset{}
	}
	if _, exists := t.doc.Assets[key]; !exists {
		// Re-wrapped canonically (76 columns, AD-9) by writeAssets at
		// serialize time regardless of how it is stored here; a single
		// element is sufficient in memory.
		t.doc.Assets[key] = template.Asset{MediaType: mediaType, Data: []string{base64.StdEncoding.EncodeToString(decoded)}}
	}
	element.Asset = template.Presence[string]{Set: true, Value: key}
	if previousKey != "" && previousKey != key && !assetKeyReferenced(t, previousKey) {
		delete(t.doc.Assets, previousKey)
	}
	return canvas(t)
}

// assetKeyReferenced reports whether anything in the document still names key —
// an image ELEMENT, across every band, or a FONT CHAIN entry. D-5.13.3: orphan
// collection is scoped to exactly the one
// key this command just repointed away from, never a document-wide sweep —
// a document may legally carry an asset no element references (RP-11's
// positive control, render_image_test.go), and this command must never
// silently remove one it did not just orphan.
//
// This is the SAFETY half of a delete: under-reporting a reference here
// deletes a live asset with no compile error to announce it. It walks the
// same three top-level band element lists (pageHeader/content/pageFooter)
// findComponent (component_commands.go) and addCanvasImagePaint
// (page_setup.go) enumerate — correct for today's model, where images in
// table cells are explicitly out of scope (AC4's exclusions, Finding 17,
// review of 2026-08-29). If a later story places an image anywhere else
// (a table cell, most likely), this walk, findComponent's and
// addCanvasImagePaint's ALL need the new location added together — there
// is no single shared element-enumeration helper today, so update all
// three by hand rather than assuming one covers the others.
//
// DW-80 IS FIXED HERE (Story 8.6), AND IT WAS A REAL HOLE, NOT A LATENT ONE.
// Until this story the only `true` arm required `el.Type == ElementImage`, and
// the walk never read `t.doc.Fonts` at all — so this function answered FALSE
// for every font asset in every document, however many chains named it. Nothing
// called it for a font, which is why it was survivable; this story's orphan
// drop is the first caller that does, and shipping that drop over the old walk
// would have deleted a face a live chain was still drawing with. The DELETE is
// the dangerous direction: under-reporting a reference here removes a live
// asset, and no compile error announces it.
//
// THE THREE-WALK WARNING ABOVE DOES NOT EXTEND TO THIS ARM, and that is a
// measurement rather than an omission. `findComponent` and `addCanvasImagePaint`
// enumerate ELEMENTS, because an image is placed by one; a font asset is named
// by an entry of a document-level map that holds no elements at all, so there
// is no third location for those two to gain. What DOES pair with this arm is
// fontChainReferences, which walks the same map for the same safety reason from
// the other direction (which elements name a chain).
func assetKeyReferenced(t *Template, key string) bool {
	for _, elements := range [][]template.Element{t.doc.Bands.PageHeader.Elements, contentElements(t), t.doc.Bands.PageFooter.Elements} {
		for _, el := range elements {
			if el.Type == template.ElementImage && el.Asset.Set && !el.Asset.Null && el.Asset.Value == key {
				return true
			}
		}
	}
	// SORTED KEYS, not a bare map range (AD-1, NFR1.d). The ANSWER here does
	// not depend on the order — it is an existence question — but the rule is
	// module-wide and absolute for a reason: an iteration order that happens
	// not to matter today is the one that quietly starts mattering when
	// somebody returns the first match instead of a bool.
	for _, name := range slices.Sorted(maps.Keys(t.doc.Fonts)) {
		for _, entry := range t.doc.Fonts[name] {
			// Embedded() is THE discriminant (template.FontChainEntry); a
			// bare `entry.AssetKey == key` would be the same test written a
			// second time, and a face entry can never carry an asset key
			// anyway — the partition is the type's own invariant.
			if entry.Embedded() && entry.AssetKey == key {
				return true
			}
		}
	}
	return false
}

func updateComponentPropertiesInPlace(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	if err := componentFields(raw, 4); err != nil {
		return designer.CanvasProjection{}, err
	}
	idsRaw, ok := raw["ids"]
	if !ok {
		return designer.CanvasProjection{}, componentFailure("", "component.ids", "component ids are required")
	}
	var ids []string
	if json.Unmarshal(idsRaw, &ids) != nil || len(ids) == 0 {
		return designer.CanvasProjection{}, componentFailure("", "component.ids", "component ids must be a non-empty string array")
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if id == "" || seen[id] {
			return designer.CanvasProjection{}, componentFailure(id, "component.ids", "component ids must be unique non-empty strings")
		}
		seen[id] = true
	}
	changesRaw, ok := raw["changes"]
	if !ok {
		return designer.CanvasProjection{}, componentFailure("", "component.changes", "component changes are required")
	}
	var changes map[string]json.RawMessage
	if json.Unmarshal(changesRaw, &changes) != nil || len(changes) == 0 {
		return designer.CanvasProjection{}, componentFailure("", "component.changes", "component changes must be a non-empty object")
	}
	if len(ids) > 1 {
		if _, ok := changes["value"]; ok {
			return designer.CanvasProjection{}, componentFailure("", "component.value", "text value cannot be edited across a selection")
		}
	}
	for _, id := range ids {
		_, band, _, element, err := findComponent(t, id)
		if err != nil {
			return designer.CanvasProjection{}, componentFailure(id, "component.id", "component was not found")
		}
		if err := applyPropertyChanges(t, element, changes); err != nil {
			return designer.CanvasProjection{}, componentFailure(id, "component."+propertyPath(changes), err.Error())
		}
		width, height := projectedSize(*element)
		if err := containComponent(band, element.X, element.Y, width, height); err != nil {
			return designer.CanvasProjection{}, componentFailure(id, "component.geometry", err.Error())
		}
		if err := refuseSectionBreakStraddle(t, band.Name, *element, "component."+propertyPath(changes)); err != nil {
			return designer.CanvasProjection{}, err
		}
	}
	// Validate authored expressions before projection bounds can mask their
	// located cause. The caller still installs this copy only after reparse.
	if _, err := validateAndDeriveExpressions(t.doc); err != nil {
		return designer.CanvasProjection{}, err
	}
	return canvas(t)
}

func propertyPath(changes map[string]json.RawMessage) string {
	// This is a fixed command vocabulary, so use its canonical order rather
	// than ranging a map (diagnostic location must be repeatable too).
	for _, key := range []string{"x", "y", "width", "height", "value", "expression", "visibleIf", "fontFamily", "fontSize", "lineSpacing", "bold", "italic", "align", "valign", "color", "background", "borderWidth", "borderColor", "borderEdges", "paddingTop", "paddingRight", "paddingBottom", "paddingLeft", "errorCorrection"} {
		if _, ok := changes[key]; ok {
			return key
		}
	}
	return "changes"
}

func propertyChange(raw json.RawMessage) (string, json.RawMessage, error) {
	var value map[string]json.RawMessage
	if json.Unmarshal(raw, &value) != nil || (len(value) != 1 && len(value) != 2) {
		return "", nil, fmt.Errorf("property change must be an operation object")
	}
	op, ok := value["op"]
	var operation string
	if !ok || json.Unmarshal(op, &operation) != nil || (operation != "set" && operation != "clear" && operation != "null") {
		return "", nil, fmt.Errorf("property operation must be set, clear, or null")
	}
	if operation == "clear" || operation == "null" {
		if len(value) != 1 {
			return "", nil, fmt.Errorf("clear property operation cannot carry a value")
		}
		return operation, nil, nil
	}
	v, ok := value["value"]
	if !ok || len(value) != 2 {
		return "", nil, fmt.Errorf("set property operation requires exactly one value")
	}
	return operation, v, nil
}

func propertyString(raw json.RawMessage) (string, error) {
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return "", fmt.Errorf("property value must be a string")
	}
	return value, nil
}
func propertyBool(raw json.RawMessage) (bool, error) {
	var value bool
	if json.Unmarshal(raw, &value) != nil {
		return false, fmt.Errorf("property value must be a boolean")
	}
	return value, nil
}
func propertyLength(raw json.RawMessage, key string) (geom.Length, error) {
	return lengthField(map[string]json.RawMessage{key: raw}, key)
}
func styleFor(element *template.Element) *template.Style {
	if !element.Style.Set || element.Style.Null {
		element.Style = template.Presence[template.Style]{Set: true}
	}
	return &element.Style.Value
}

func applyPropertyChanges(t *Template, element *template.Element, changes map[string]json.RawMessage) error {
	allowed := map[string]bool{"x": true, "y": true, "visibleIf": true}
	propertyOrder := []string{"x", "y", "width", "height", "value", "expression", "visibleIf", "fontFamily", "fontSize", "lineSpacing", "bold", "italic", "align", "valign", "color", "background", "borderWidth", "borderColor", "borderEdges", "paddingTop", "paddingRight", "paddingBottom", "paddingLeft", "errorCorrection"}
	if element.Type != template.ElementTable {
		allowed["width"], allowed["height"] = true, true
	}
	if element.Type == template.ElementText || isCodeElement(element.Type) {
		allowed["value"] = true
		allowed["expression"] = true
	}
	if element.Type == template.ElementQRCode {
		// spec-barcode-qr-elements: the one option a QR code carries.
		allowed["errorCorrection"] = true
	}
	if element.Type == template.ElementText || element.Type == template.ElementImage || element.Type == template.ElementTable || element.Type == template.ElementLine || element.Type == template.ElementRect {
		for _, key := range []string{"background", "borderWidth", "borderColor", "borderEdges"} {
			allowed[key] = true
		}
	}
	if element.Type == template.ElementTable {
		// D-12.4.1: `padding` is consumed by a table's cell chrome and by
		// nothing else on any render path, so a table is the only kind that
		// may be commanded to change it. The asymmetry is deliberate: a
		// loaded document KEEPS and round-trips padding on any kind, and
		// RENDERS it where a table consumes it — the engine honours what it
		// is given — while the designer refuses to author, on the four kinds
		// that would never paint it, a value nothing would ever read.
		for _, key := range []string{"paddingTop", "paddingRight", "paddingBottom", "paddingLeft"} {
			allowed[key] = true
		}
	}
	if element.Type == template.ElementText || element.Type == template.ElementTable {
		// Story 10.1: `color` is the ink text prints in, so it is offered
		// exactly where text is — never on a rect, line or image, which
		// carry no glyphs for it to colour.
		for _, key := range []string{"fontFamily", "fontSize", "lineSpacing", "bold", "italic", "align", "valign", "color"} {
			allowed[key] = true
		}
	}
	known := 0
	for _, key := range propertyOrder {
		if _, ok := changes[key]; ok {
			known++
		}
	}
	if known != len(changes) {
		return fmt.Errorf("property is not editable")
	}
	for _, key := range propertyOrder {
		change, present := changes[key]
		if !present {
			continue
		}
		if !allowed[key] {
			return fmt.Errorf("property %s is not editable for %s", key, element.Type)
		}
		op, value, err := propertyChange(change)
		if err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		clear := op == "clear"
		setNull := op == "null"
		switch key {
		case "x", "y", "width", "height", "fontSize", "borderWidth", "paddingTop", "paddingRight", "paddingBottom", "paddingLeft":
			if setNull {
				return fmt.Errorf("%s does not support null", key)
			}
			if clear && (key == "x" || key == "y" || key == "width" || key == "height") {
				return fmt.Errorf("%s cannot be cleared", key)
			}
			var length geom.Length
			if !clear {
				length, err = propertyLength(value, key)
				if err != nil {
					return fmt.Errorf("%s: %w", key, err)
				}
				if (key == "width" || key == "height" || key == "fontSize" || key == "borderWidth") && length <= 0 {
					return fmt.Errorf("%s must be positive", key)
				}
				if stringsContainsPlaceholder(string(value)) {
					return fmt.Errorf("%s must not contain a placeholder", key)
				}
			}
			switch key {
			case "x":
				element.X = length
			case "y":
				element.Y = length
			case "width":
				element.Width = template.Presence[geom.Length]{Set: true, Value: length}
			case "height":
				element.Height = template.Presence[geom.Length]{Set: true, Value: length}
			case "fontSize":
				st := styleFor(element)
				if clear {
					st.FontSize = template.Presence[geom.Length]{}
				} else {
					st.FontSize = template.Presence[geom.Length]{Set: true, Value: length}
				}
			case "borderWidth":
				st := styleFor(element)
				if !st.Border.Set || st.Border.Null {
					st.Border = template.Presence[template.Border]{Set: true}
				}
				if clear {
					st.Border.Value.Width = template.Presence[geom.Length]{}
				} else {
					st.Border.Value.Width = template.Presence[geom.Length]{Set: true, Value: length}
				}
			default:
				st := styleFor(element)
				if !st.Padding.Set || st.Padding.Null {
					st.Padding = template.Presence[template.Padding]{Set: true}
				}
				target := map[string]*template.Presence[geom.Length]{"paddingTop": &st.Padding.Value.Top, "paddingRight": &st.Padding.Value.Right, "paddingBottom": &st.Padding.Value.Bottom, "paddingLeft": &st.Padding.Value.Left}[key]
				if clear {
					*target = template.Presence[geom.Length]{}
				} else {
					*target = template.Presence[geom.Length]{Set: true, Value: length}
				}
			}
		case "value", "expression", "visibleIf", "fontFamily", "align", "valign", "color", "background", "borderColor", "errorCorrection":
			var text string
			if !clear && !setNull {
				text, err = propertyString(value)
				if err != nil {
					return fmt.Errorf("%s: %w", key, err)
				}
				if isCodeElement(element.Type) && (key == "value" || key == "expression") {
					// The designer spells control characters as \r, \n and
					// \\ outside {{ }}; the document stores the characters.
					decoded, derr := decodeBarcodeEscapes(text)
					if derr != nil {
						return fmt.Errorf("%s: %w", key, derr)
					}
					text = decoded
				}
				if key != "value" && key != "expression" && stringsContainsPlaceholder(text) {
					return fmt.Errorf("%s must not contain a placeholder", key)
				}
			}
			switch key {
			case "value":
				if clear || setNull {
					return fmt.Errorf("value cannot be cleared")
				}
				if stringsContainsPlaceholder(text) {
					// Direct bindings are authored only by bindComponentScalar. This
					// command deliberately remains useful for literal text, but cannot
					// become a second, typed expression route.
					return fmt.Errorf("value must not contain a placeholder; choose a path in the Data panel")
				}
				element.Value = template.Presence[string]{Set: true, Value: text}
			case "expression":
				if clear || setNull || !stringsContainsPlaceholder(text) {
					return fmt.Errorf("expression must contain a template placeholder")
				}
				element.Value = template.Presence[string]{Set: true, Value: text}
			case "visibleIf":
				if clear {
					element.VisibleIf = template.Presence[string]{}
				} else if setNull {
					element.VisibleIf = template.Presence[string]{Set: true, Null: true}
				} else {
					element.VisibleIf = template.Presence[string]{Set: true, Value: text}
				}
			case "fontFamily":
				if setNull {
					return fmt.Errorf("fontFamily does not support null")
				}
				if !clear && !knownFontFamily(t, text) {
					return fmt.Errorf("fontFamily must name a declared non-empty font chain")
				}
				st := styleFor(element)
				if clear {
					st.FontFamily = template.Presence[string]{}
				} else {
					st.FontFamily = template.Presence[string]{Set: true, Value: text}
				}
			case "align":
				if setNull {
					return fmt.Errorf("align does not support null")
				}
				st := styleFor(element)
				if clear {
					st.Align = template.Presence[string]{}
				} else {
					// Story 7.3. This arm set style.align to WHATEVER
					// STRING ARRIVED — pre-existing, and harmless only
					// while one closed set served both vocabularies.
					// With more than one live it is the one remaining
					// place they could be conflated, so it validates
					// through the closed sets' own exported predicates,
					// and names the legal values from the matching
					// ordered slice rather than from a literal
					// restating it. The COLUMN arm (updateTableColumn,
					// above) keeps its own triple and still refuses
					// "justify".
					//
					// Story 7.8: SELECTED BY ELEMENT TYPE, the same way
					// decodeStyle selects. IsStyleAlign's own doc
					// comment requires this path to validate against
					// the same single source the loader does.
					//
					// MEASURED, not assumed: updateComponentProperties
					// serializes and RE-PARSES before installing, so
					// once the loader refuses a table's `justify` the
					// round trip already stops the document reaching
					// 2.0 through this door — with the generic
					// "component properties did not pass format
					// validation". What this arm adds is the REFUSAL
					// THE AUTHOR CAN ACT ON: the field named, and the
					// legal values for a table rather than for a
					// paragraph. Without it the inspector would report
					// a whole-command failure for one bad value and
					// name neither. It is also the layer that does not
					// depend on the round trip continuing to exist.
					admits, tokens := template.IsStyleAlign, template.StyleAlignTokens
					if element.Type == template.ElementTable {
						admits, tokens = template.IsTableStyleAlign, template.TableStyleAlignTokens
					}
					if !admits(text) {
						return fmt.Errorf("align must be one of %s", strings.Join(tokens, ", "))
					}
					st.Align = template.Presence[string]{Set: true, Value: text}
				}
			case "errorCorrection":
				// Modelled on align: set validates against the loader's own
				// closed set (template.IsQRErrorCorrection), clear removes the
				// key (the default M), and null is refused because the loader
				// refuses it.
				if setNull {
					return fmt.Errorf("errorCorrection does not support null")
				}
				if clear {
					element.ErrorCorrection = template.Presence[string]{}
				} else {
					if !template.IsQRErrorCorrection(text) {
						return fmt.Errorf("errorCorrection must be one of %s", strings.Join(template.QRErrorCorrectionTokens, ", "))
					}
					element.ErrorCorrection = template.Presence[string]{Set: true, Value: text}
				}
			case "valign":
				if setNull {
					return fmt.Errorf("valign does not support null")
				}
				st := styleFor(element)
				if clear {
					st.Valign = template.Presence[string]{}
				} else {
					st.Valign = template.Presence[string]{Set: true, Value: text}
				}
			case "color":
				st := styleFor(element)
				if clear {
					st.Color = template.Presence[string]{}
				} else if setNull {
					st.Color = template.Presence[string]{Set: true, Null: true}
				} else if !validPropertyColor(text) {
					return fmt.Errorf("color must be a #RRGGBB colour")
				} else {
					st.Color = template.Presence[string]{Set: true, Value: text}
				}
			case "background":
				st := styleFor(element)
				if clear {
					st.Background = template.Presence[string]{}
				} else if setNull {
					st.Background = template.Presence[string]{Set: true, Null: true}
				} else if !validPropertyColor(text) {
					return fmt.Errorf("background must be a #RRGGBB colour")
				} else {
					st.Background = template.Presence[string]{Set: true, Value: text}
				}
			case "borderColor":
				if setNull {
					return fmt.Errorf("borderColor does not support null")
				}
				if !clear && !validPropertyColor(text) {
					return fmt.Errorf("borderColor must be a #RRGGBB colour")
				}
				st := styleFor(element)
				if !st.Border.Set || st.Border.Null {
					st.Border = template.Presence[template.Border]{Set: true}
				}
				if clear {
					st.Border.Value.Color = template.Presence[string]{}
				} else {
					st.Border.Value.Color = template.Presence[string]{Set: true, Value: text}
				}
			}
		case "lineSpacing":
			// NOT propertyLength. That decoder reads POINTS and bounds
			// them by MaxCanvasMillipoints; lineSpacing is a
			// dimensionless ratio with its own domain, and borrowing a
			// length decoder would give the inspector a different notion
			// of a legal value from the one the file path enforces.
			//
			// D-7.2.3's "a value refused in a file is refused in the
			// inspector for the SAME reason" is satisfied by calling the
			// SAME function the loader calls — template.DecodeLineSpacing
			// — not by mirroring its bounds here.
			if setNull {
				return fmt.Errorf("%s does not support null", key)
			}
			st := styleFor(element)
			if clear {
				st.LineSpacing = template.Presence[int64]{}
				continue
			}
			thousandths, err := template.DecodeLineSpacingRaw(value)
			if err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
			st.LineSpacing = template.Presence[int64]{Set: true, Value: thousandths}
		case "bold", "italic":
			if setNull {
				return fmt.Errorf("%s does not support null", key)
			}
			if clear {
				if key == "bold" {
					styleFor(element).Bold = template.Presence[bool]{}
				} else {
					styleFor(element).Italic = template.Presence[bool]{}
				}
				continue
			}
			flag, err := propertyBool(value)
			if err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
			if key == "bold" {
				styleFor(element).Bold = template.Presence[bool]{Set: true, Value: flag}
			} else {
				styleFor(element).Italic = template.Presence[bool]{Set: true, Value: flag}
			}
		case "borderEdges":
			if setNull {
				return fmt.Errorf("borderEdges does not support null")
			}
			if clear {
				st := styleFor(element)
				if st.Border.Null {
					st.Border = template.Presence[template.Border]{}
				} else if st.Border.Set {
					st.Border.Value.Edges = template.Presence[[]string]{}
				}
				continue
			}
			var edges []string
			if json.Unmarshal(value, &edges) != nil || len(edges) == 0 {
				return fmt.Errorf("borderEdges must be a non-empty string array")
			}
			st := styleFor(element)
			if !st.Border.Set || st.Border.Null {
				st.Border = template.Presence[template.Border]{Set: true}
			}
			st.Border.Value.Edges = template.Presence[[]string]{Set: true, Value: edges}
		}
	}
	cleanupEmptyStyle(element)
	return nil
}

// validPropertyColor is the command door's colour check: the loader's own
// predicate, so a value refused in a file is refused here too.
func validPropertyColor(value string) bool {
	return template.IsHexColour(value)
}

func knownFontFamily(t *Template, value string) bool {
	_, ok := t.doc.Fonts.Chain(value)
	return ok
}

func cleanupEmptyStyle(element *template.Element) {
	if !element.Style.Set || element.Style.Null {
		return
	}
	style := &element.Style.Value
	if style.Border.Set && !style.Border.Null {
		border := style.Border.Value
		if !border.Color.Set && !border.Width.Set && !border.Edges.Set && len(border.Extra) == 0 {
			style.Border = template.Presence[template.Border]{}
		}
	}
	if style.Padding.Set && !style.Padding.Null {
		padding := style.Padding.Value
		if !padding.Top.Set && !padding.Right.Set && !padding.Bottom.Set && !padding.Left.Set && len(padding.Extra) == 0 {
			style.Padding = template.Presence[template.Padding]{}
		}
	}
	if !style.Align.Set && !style.Background.Set && !style.Bold.Set && !style.Color.Set && !style.Italic.Set && !style.Border.Set && !style.FontFamily.Set && !style.FontSize.Set && !style.LineSpacing.Set && !style.Padding.Set && !style.Valign.Set && len(style.Extra) == 0 {
		element.Style = template.Presence[template.Style]{}
	}
}

func stringsContainsPlaceholder(value string) bool {
	return bytes.Contains([]byte(value), []byte("{{")) || bytes.Contains([]byte(value), []byte("}}"))
}

func componentFields(raw map[string]json.RawMessage, want int) error {
	if len(raw) != want {
		return fmt.Errorf("folio8: component command has unknown or missing fields")
	}
	return nil
}

func commandString(raw map[string]json.RawMessage, name string) (string, error) {
	v, ok := raw[name]
	if !ok {
		return "", fmt.Errorf("folio8: %s is required", name)
	}
	var out string
	if json.Unmarshal(v, &out) != nil || out == "" {
		return "", fmt.Errorf("folio8: %s must be a non-empty string", name)
	}
	return out, nil
}

func commandBool(raw map[string]json.RawMessage, name string) (bool, error) {
	v, ok := raw[name]
	if !ok {
		return false, fmt.Errorf("folio8: %s is required", name)
	}
	var out bool
	if json.Unmarshal(v, &out) != nil {
		return false, fmt.Errorf("folio8: %s must be a boolean", name)
	}
	return out, nil
}

func componentLength(raw map[string]json.RawMessage, name string, snap bool) (geom.Length, error) {
	v, err := lengthField(raw, name)
	if err != nil {
		return 0, fmt.Errorf("folio8: component.%s: %w", name, err)
	}
	if snap {
		return snapField(name, v)
	}
	return v, nil
}

func snapField(name string, value geom.Length) (geom.Length, error) {
	snapped, valid := snapToGrid(value)
	if !valid {
		return 0, fmt.Errorf("folio8: component.%s overflows grid snapping", name)
	}
	return snapped, nil
}

func commandBand(raw map[string]json.RawMessage) (string, *template.Band, error) {
	name, err := commandString(raw, "band")
	if err != nil {
		return "", nil, err
	}
	return name, nil, nil
}

func bandByName(t *Template, name string) (*template.Band, designer.CanvasBand, error) {
	projection, err := canvas(t)
	if err != nil {
		return nil, designer.CanvasBand{}, err
	}
	for _, projected := range projection.Bands {
		if projected.Name != name {
			continue
		}
		switch name {
		case bandPageHeader:
			return &t.doc.Bands.PageHeader, projected, nil
		case bandContent:
			// Commands that create into `content` target page 1.
			return firstContentBand(t), projected, nil
		case bandPageFooter:
			return &t.doc.Bands.PageFooter, projected, nil
		}
	}
	return nil, designer.CanvasBand{}, fmt.Errorf("folio8: component.band must be pageHeader, content, or pageFooter")
}

func createComponent(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	fields := 9
	if _, ok := raw["page"]; ok {
		fields++
	}
	if err := componentFields(raw, fields); err != nil {
		return designer.CanvasProjection{}, err
	}
	kind, err := commandString(raw, "type")
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	elementType := template.ElementType(kind)
	if !paletteElementType(elementType) {
		return designer.CanvasProjection{}, fmt.Errorf("folio8: component.type must be text, image, table, line, rect, barcode, or qrcode")
	}
	bandName, _, err := commandBand(raw)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	snap, err := commandBool(raw, "snap")
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	x, err := componentLength(raw, "x", snap)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	y, err := componentLength(raw, "y", snap)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	width, err := componentLength(raw, "width", snap)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	height, err := componentLength(raw, "height", snap)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	page, hasPage, err := optionalPageField(t, raw)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	if hasPage && bandName != bandContent {
		return designer.CanvasProjection{}, contentOnlyPage(bandName, "")
	}
	return createComponentInBand(t, elementType, bandName, page, x, y, width, height)
}

// dropComponent resolves a document point in Go. Band rectangles use the
// half-open convention [x, x+width) × [y, y+height): a shared boundary belongs
// to the next band, and a page edge outside the last band is rejected.
// The size a dropped component starts at, before any property edit. An image
// starts larger than the rest: until a file is chosen its box carries the
// designer's empty-state placeholder — an icon above a label — which a
// 72x24 box cuts in half. Both sizes sit on the 6pt grid, so a snapped drop
// keeps them exactly.
const dropWidth, dropHeight geom.Length = 72000, 24000
const imageDropWidth, imageDropHeight geom.Length = 96000, 48000

// Story 9.2: a line's declared HEIGHT is its thickness — element_box.go
// paints a line as a filled bar of its declared box — so a line drops as a
// 1pt rule rather than as a 72x24 slab. Off the 6pt grid on purpose: a
// rule's thickness is not a position, and snapping applies to x/y alone.
const lineDropHeight geom.Length = 1000

const tableDropHeight geom.Length = 24000

// A barcode drops wide enough for its starter value to print at a scannable
// module width (216 pt / 110 modules = 1,963 mp, above 0.25 mm), and tall
// enough to scan. Both sit on the 6pt grid.
const barcodeDropWidth, barcodeDropHeight geom.Length = 216000, 48000

// barcodeStarterValue is what a newly placed barcode encodes until the author
// edits or binds it: ten digits, a compact set-C symbol.
const barcodeStarterValue = "1234567890"

// A QR code drops as a 72 pt square: its starter value is a version-1 symbol
// (21 modules plus an 8-module quiet zone), so modules are 2,482 mp, well
// above 0.5 mm. Both sides sit on the 6pt grid.
const qrcodeDropSide geom.Length = 72000

// qrcodeStarterValue is what a newly placed QR code encodes until the author
// edits or binds it.
const qrcodeStarterValue = "folio8"

// paletteElementType is the closed set of kinds a create or drop command may
// place: every element type the format declares.
func paletteElementType(elementType template.ElementType) bool {
	switch elementType {
	case template.ElementText, template.ElementImage, template.ElementTable, template.ElementLine, template.ElementRect, template.ElementBarcode, template.ElementQRCode:
		return true
	}
	return false
}

func dropComponent(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	fields := 6
	if _, ok := raw["page"]; ok {
		fields++
	}
	if err := componentFields(raw, fields); err != nil {
		return designer.CanvasProjection{}, err
	}
	kind, err := commandString(raw, "type")
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	elementType := template.ElementType(kind)
	if !paletteElementType(elementType) {
		return designer.CanvasProjection{}, componentFailure("", "component.type", "component type must be text, image, table, line, rect, barcode, or qrcode")
	}
	snap, err := commandBool(raw, "snap")
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	pageX, err := componentLength(raw, "x", false)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	pageY, err := componentLength(raw, "y", false)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	// The point is a sheet point of the target page; the band rectangles are
	// the same on every page, so only which page's column receives it changes.
	page, hasPage, err := optionalPageField(t, raw)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	_, projected, err := hitTestBand(t, pageX, pageY)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	if hasPage && projected.Name != bandContent {
		return designer.CanvasProjection{}, contentOnlyPage(projected.Name, "")
	}
	width, height := dropWidth, dropHeight
	if elementType == template.ElementImage {
		width, height = imageDropWidth, imageDropHeight
	}
	if elementType == template.ElementLine {
		height = lineDropHeight
	}
	if elementType == template.ElementBarcode {
		width, height = barcodeDropWidth, barcodeDropHeight
	}
	if elementType == template.ElementQRCode {
		width, height = qrcodeDropSide, qrcodeDropSide
	}
	x, y := pageX-geom.Length(projected.X), pageY-geom.Length(projected.Y)
	if elementType == template.ElementTable {
		// Snap and containment must see the table's real column geometry,
		// including drops near the right edge of a non-grid-width band.
		x, width, height = 0, geom.Length(projected.Width), tableDropHeight
	}
	unsnappedX, unsnappedY := x, y
	fitImage := elementType == template.ElementImage && slices.Contains(bandsCappingVertically, projected.Name)
	if snap {
		var valid bool
		x, valid = snapToGrid(x)
		if !valid {
			return designer.CanvasProjection{}, componentFailure("", "component.x", "component x overflows grid snapping")
		}
		y, valid = snapToGrid(y)
		if !valid {
			return designer.CanvasProjection{}, componentFailure("", "component.y", "component y overflows grid snapping")
		}
		if !fitImage && containComponent(projected, unsnappedX, unsnappedY, width, height) == nil {
			x = containEdge(x, geom.Length(projected.Width)-width)
			y = containEdgeY(projected, y, geom.Length(projected.Height)-height)
		}
	}
	// An empty image placeholder keeps its drop origin and shrinks only the
	// dimensions that exceed the remaining header/footer space. A snapped
	// point must stay strictly inside the band so both dimensions stay positive.
	// Explicit geometry and the content column keep their existing contracts.
	if fitImage {
		if snap {
			x = min(x, floorToGrid(geom.Length(projected.Width)-1))
			y = min(y, floorToGrid(geom.Length(projected.Height)-1))
		}
		width = min(width, geom.Length(projected.Width)-x)
		height = min(height, geom.Length(projected.Height)-y)
	}
	return createComponentInBand(t, elementType, projected.Name, page, x, y, width, height)
}

// createComponentInBand creates into bandName; for content, into page's
// column (0 is page 1).
func createComponentInBand(t *Template, elementType template.ElementType, bandName string, page int, x, y, width, height geom.Length) (designer.CanvasProjection, error) {
	band, projected, err := bandByName(t, bandName)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	if bandName == bandContent {
		band = t.doc.ContentBands()[page]
	}
	idsNeeded := int64(1)
	if elementType == template.ElementTable {
		idsNeeded = 2
	}
	if t.doc.NextID <= 0 || t.doc.NextID > (1<<63-1)-idsNeeded {
		return designer.CanvasProjection{}, fmt.Errorf("folio8: nextId cannot allocate another component")
	}
	// Allocate against a local cursor; a refusal consumes neither the table's
	// id nor its starter column's id.
	ids := template.Document{NextID: t.doc.NextID}
	element := template.Element{ID: template.AllocateElementID(&ids), Type: elementType, X: x, Y: y}
	ids.NextID++
	if elementType == template.ElementTable {
		x, width, height = 0, geom.Length(projected.Width), tableDropHeight
		element.X = x
		element.Width = template.Presence[geom.Length]{Set: true, Value: width}
		column := template.Column{ID: template.AllocateElementID(&ids), Proportion: template.Presence[int64]{Set: true, Value: template.ProportionUnit}}
		ids.NextID++
		element.Table = template.Presence[template.TableExt]{Set: true, Value: template.TableExt{Bind: "items[]", Columns: []template.Column{column}, HeaderHeight: height}}
		// Tables retain their dedicated placement behavior; their starter
		// proportion receives the band's available authored total.
	} else {
		if width <= 0 || height <= 0 {
			return designer.CanvasProjection{}, fmt.Errorf("folio8: component.width and component.height must be positive")
		}
		element.Width = template.Presence[geom.Length]{Set: true, Value: width}
		element.Height = template.Presence[geom.Length]{Set: true, Value: height}
		if elementType == template.ElementText {
			element.Value = template.Presence[string]{Set: true, Value: "Text"}
		}
		if elementType == template.ElementBarcode {
			element.Value = template.Presence[string]{Set: true, Value: barcodeStarterValue}
		}
		if elementType == template.ElementQRCode {
			element.Value = template.Presence[string]{Set: true, Value: qrcodeStarterValue}
		}
		// Story 9.2: a line and a rect ARE their box — they carry no text
		// and no asset — so a placed one with no style would render, and
		// paint on the canvas, as nothing at all. Each starts with the one
		// declaration that makes it the shape its palette entry names: a
		// line is a filled rule, a rect is an outlined box. Both are
		// ordinary style values the author edits or clears like any other.
		if elementType == template.ElementLine {
			styleFor(&element).Background = template.Presence[string]{Set: true, Value: "#000000"}
		}
		if elementType == template.ElementRect {
			styleFor(&element).Border = template.Presence[template.Border]{Set: true, Value: template.Border{
				Color: template.Presence[string]{Set: true, Value: "#000000"},
				Width: template.Presence[geom.Length]{Set: true, Value: 1000},
			}}
		}
		if elementType == template.ElementImage {
			// A placed image starts empty: the author positions and sizes the
			// box first and chooses the file through the inspector, which is
			// the state the design draws as a dashed placeholder. The asset
			// field is present and null rather than absent, so the document
			// still declares the box as an image and Render draws nothing for
			// it until a file is set.
			element.Asset = template.Presence[string]{Set: true, Null: true}
		}
	}
	// Text and tables need a declared font chain as soon as their text is
	// rendered, including a new table's edited header or populated items.
	if elementType == template.ElementText || elementType == template.ElementTable {
		if chain := defaultFontFamily(t); chain != "" {
			styleFor(&element).FontFamily = template.Presence[string]{Set: true, Value: chain}
		}
	}
	if err := containComponent(projected, x, y, width, height); err != nil {
		return designer.CanvasProjection{}, componentFailure("", "component.geometry", err.Error())
	}
	if err := refuseSectionBreakStraddleOnPage(t, bandName, page, element, "component.geometry"); err != nil {
		return designer.CanvasProjection{}, err
	}
	previousElements, previousID := band.Elements, t.doc.NextID
	band.Elements = append(band.Elements, element)
	t.doc.NextID = ids.NextID
	projection, err := canvas(t)
	if err != nil {
		band.Elements, t.doc.NextID = previousElements, previousID
		return designer.CanvasProjection{}, err
	}
	return projection, nil
}

// defaultFontFamily names the chain a newly created text or table adopts: the
// first declared non-empty chain in sorted key order. Sorted rather than
// ranged (ScanMapRange), so a document's declared fonts pick the same chain on
// every run. An empty result means the document declares no usable chain and
// there is nothing to adopt; fontFamily stays absent exactly as before.
func defaultFontFamily(t *Template) string {
	for _, name := range slices.Sorted(maps.Keys(t.doc.Fonts)) {
		if _, ok := t.doc.Fonts.Chain(name); ok {
			return name
		}
	}
	return ""
}

func hitTestBand(t *Template, x, y geom.Length) (*template.Band, designer.CanvasBand, error) {
	projection, err := canvas(t)
	if err != nil {
		return nil, designer.CanvasBand{}, err
	}
	for _, band := range projection.Bands {
		left, top := geom.Length(band.X), geom.Length(band.Y)
		if x < left || x >= left+geom.Length(band.Width) || y < top || y >= top+geom.Length(band.Height) {
			continue
		}
		switch band.Name {
		case bandPageHeader:
			return &t.doc.Bands.PageHeader, band, nil
		case bandContent:
			// Page 1's band; a drop with a target page is redirected by
			// createComponentInBand.
			return firstContentBand(t), band, nil
		case bandPageFooter:
			return &t.doc.Bands.PageFooter, band, nil
		}
	}
	return nil, designer.CanvasBand{}, componentFailure("", "component.drop", "drop point is outside a page band")
}

func findComponent(t *Template, id string) (*template.Band, designer.CanvasBand, int, *template.Element, error) {
	for _, name := range []string{bandPageHeader, bandContent, bandPageFooter} {
		band, projected, err := bandByName(t, name)
		if err != nil {
			return nil, designer.CanvasBand{}, 0, nil, err
		}
		// SPEC-multi-pages: an element is found on whichever designed page
		// holds it, and returned with that page's band.
		candidates := []*template.Band{band}
		if name == bandContent {
			candidates = t.doc.ContentBands()
		}
		for _, band := range candidates {
			for index := range band.Elements {
				if string(band.Elements[index].ID) == id {
					return band, projected, index, &band.Elements[index], nil
				}
			}
		}
	}
	return nil, designer.CanvasBand{}, 0, nil, fmt.Errorf("folio8: component %q was not found", id)
}

func moveComponent(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	if err := componentFields(raw, 6); err != nil {
		return designer.CanvasProjection{}, err
	}
	id, err := commandString(raw, "id")
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	snap, err := commandBool(raw, "snap")
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	unsnappedX, err := componentLength(raw, "x", false)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	unsnappedY, err := componentLength(raw, "y", false)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	x, y := unsnappedX, unsnappedY
	if snap {
		if x, err = snapField("x", unsnappedX); err != nil {
			return designer.CanvasProjection{}, err
		}
		if y, err = snapField("y", unsnappedY); err != nil {
			return designer.CanvasProjection{}, err
		}
	}
	_, projected, _, element, err := findComponent(t, id)
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "component.id", "component was not found")
	}
	width, height := projectedSize(*element)
	if snap && containComponent(projected, unsnappedX, unsnappedY, width, height) == nil {
		x = containEdge(x, geom.Length(projected.Width)-width)
		y = containEdgeY(projected, y, geom.Length(projected.Height)-height)
	}
	if err := containComponent(projected, x, y, width, height); err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "component.geometry", err.Error())
	}
	candidate := *element
	candidate.X, candidate.Y = x, y
	if err := refuseSectionBreakStraddle(t, projected.Name, candidate, "component.geometry"); err != nil {
		return designer.CanvasProjection{}, err
	}
	element.X, element.Y = x, y
	return canvas(t)
}

func resizeComponent(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	if err := componentFields(raw, 6); err != nil {
		return designer.CanvasProjection{}, err
	}
	id, err := commandString(raw, "id")
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	snap, err := commandBool(raw, "snap")
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	width, err := componentLength(raw, "width", snap)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	height, err := componentLength(raw, "height", snap)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	_, projected, _, element, err := findComponent(t, id)
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "component.id", "component was not found")
	}
	if element.Type == template.ElementTable {
		return designer.CanvasProjection{}, componentFailure(id, "component.geometry", "table has derived geometry and cannot be resized")
	}
	if width <= 0 || height <= 0 {
		return designer.CanvasProjection{}, componentFailure(id, "component.geometry", "component width and height must be positive")
	}
	if err := containComponent(projected, element.X, element.Y, width, height); err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "component.geometry", err.Error())
	}
	candidate := *element
	candidate.Width = template.Presence[geom.Length]{Set: true, Value: width}
	candidate.Height = template.Presence[geom.Length]{Set: true, Value: height}
	if err := refuseSectionBreakStraddle(t, projected.Name, candidate, "component.geometry"); err != nil {
		return designer.CanvasProjection{}, err
	}
	element.Width = template.Presence[geom.Length]{Set: true, Value: width}
	element.Height = template.Presence[geom.Length]{Set: true, Value: height}
	return canvas(t)
}

// setComponentBounds is one rectangle, not a move followed by a resize. A
// resize anchored at any edge or corner other than the bottom-right moves the
// origin and the size together; sending moveComponent and resizeComponent in
// sequence would put two entries in history for one drag and would test
// containment against an intermediate rectangle the caller never asked for.
func setComponentBounds(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	if err := componentFields(raw, 8); err != nil {
		return designer.CanvasProjection{}, err
	}
	id, err := commandString(raw, "id")
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	snap, err := commandBool(raw, "snap")
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	unsnappedX, err := componentLength(raw, "x", false)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	unsnappedY, err := componentLength(raw, "y", false)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	unsnappedWidth, err := componentLength(raw, "width", false)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	unsnappedHeight, err := componentLength(raw, "height", false)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	_, projected, _, element, err := findComponent(t, id)
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "component.id", "component was not found")
	}
	if element.Type == template.ElementTable {
		return designer.CanvasProjection{}, componentFailure(id, "component.geometry", "table has derived geometry and cannot be resized")
	}
	// A line's short axis is its authored thickness, not a grid dimension.
	// Use the committed orientation so a short endpoint drag cannot swap axes.
	originalWidth, originalHeight := projectedSize(*element)
	horizontalLine := element.Type == template.ElementLine && originalWidth >= originalHeight
	verticalLine := element.Type == template.ElementLine && originalWidth < originalHeight
	x, y, width, height := unsnappedX, unsnappedY, unsnappedWidth, unsnappedHeight
	if snap {
		for _, field := range [4]struct {
			name  string
			value *geom.Length
		}{{"x", &x}, {"y", &y}, {"width", &width}, {"height", &height}} {
			if horizontalLine && field.name == "height" || verticalLine && field.name == "width" {
				continue
			}
			if *field.value, err = snapField(field.name, *field.value); err != nil {
				return designer.CanvasProjection{}, err
			}
		}
	}
	// Near the minimum length there may be no grid point that keeps the
	// orientation. Keep the precise proposed length rather than collapsing it.
	if horizontalLine && width < height {
		width = unsnappedWidth
	}
	if verticalLine && height <= width {
		height = unsnappedHeight
	}
	if width <= 0 || height <= 0 {
		return designer.CanvasProjection{}, componentFailure(id, "component.geometry", "component width and height must be positive")
	}
	if snap && containComponent(projected, unsnappedX, unsnappedY, unsnappedWidth, unsnappedHeight) == nil {
		if !verticalLine {
			width = containEdge(width, geom.Length(projected.Width)-x)
		}
		if !horizontalLine {
			height = containEdgeY(projected, height, geom.Length(projected.Height)-y)
		}
		x = containEdge(x, geom.Length(projected.Width)-width)
		y = containEdgeY(projected, y, geom.Length(projected.Height)-height)
	}
	if snap && containComponent(projected, unsnappedX, unsnappedY, unsnappedWidth, unsnappedHeight) == nil {
		if horizontalLine && width < height {
			x, width = unsnappedX, unsnappedWidth
		}
		if verticalLine && height <= width {
			y, height = unsnappedY, unsnappedHeight
		}
	}
	if err := containComponent(projected, x, y, width, height); err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "component.geometry", err.Error())
	}
	candidate := *element
	candidate.X, candidate.Y = x, y
	candidate.Width = template.Presence[geom.Length]{Set: true, Value: width}
	candidate.Height = template.Presence[geom.Length]{Set: true, Value: height}
	if err := refuseSectionBreakStraddle(t, projected.Name, candidate, "component.geometry"); err != nil {
		return designer.CanvasProjection{}, err
	}
	element.X, element.Y = x, y
	element.Width = template.Presence[geom.Length]{Set: true, Value: width}
	element.Height = template.Presence[geom.Length]{Set: true, Value: height}
	return canvas(t)
}

func deleteComponent(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	if err := componentFields(raw, 3); err != nil {
		return designer.CanvasProjection{}, err
	}
	id, err := commandString(raw, "id")
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	band, _, index, _, err := findComponent(t, id)
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "component.id", "component was not found")
	}
	band.Elements = append(band.Elements[:index:index], band.Elements[index+1:]...)
	return canvas(t)
}

func duplicateComponent(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	if err := componentFields(raw, 4); err != nil {
		return designer.CanvasProjection{}, err
	}
	id, err := commandString(raw, "id")
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	snap, err := commandBool(raw, "snap")
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	band, projected, _, element, err := findComponent(t, id)
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "component.id", "component was not found")
	}
	if idsNeeded := componentIDsNeeded(*element); t.doc.NextID <= 0 || t.doc.NextID > (1<<63-1)-idsNeeded {
		return designer.CanvasProjection{}, fmt.Errorf("folio8: nextId cannot allocate another component")
	}
	ids := template.Document{NextID: t.doc.NextID}
	clone := cloneComponent(*element, projected, snap, &ids)
	// The copy lands on its source's page, so that page's break judges it (the
	// clone's new id is in no page index yet).
	if err := refuseSectionBreakStraddleOnPage(t, projected.Name, contentPageIndex(t)[id], clone, "component.geometry"); err != nil {
		return designer.CanvasProjection{}, err
	}
	previousElements, previousID := band.Elements, t.doc.NextID
	band.Elements = append(band.Elements, clone)
	t.doc.NextID = ids.NextID
	projection, err := canvas(t)
	if err != nil {
		band.Elements, t.doc.NextID = previousElements, previousID
		return designer.CanvasProjection{}, err
	}
	return projection, nil
}

// componentIDsNeeded is how many document-wide ids one copy of element uses:
// its own and one per table column.
func componentIDsNeeded(element template.Element) int64 {
	if element.Type == template.ElementTable {
		return 1 + int64(len(element.Table.Value.Columns))
	}
	return 1
}

// cloneComponent is the one copy rule shared by duplicateComponent and
// duplicateComponents. It allocates from ids, so several copies in one command
// draw from one counter; the caller has already checked the counter has room.
func cloneComponent(element template.Element, projected designer.CanvasBand, snap bool, ids *template.Document) template.Element {
	clone := element
	clone.ID = template.AllocateElementID(ids)
	ids.NextID++
	if clone.Type == template.ElementTable {
		// Column IDs are document-wide identities. Copy their storage before
		// replacing IDs so the source table remains independently editable.
		clone.Table.Value.Columns = slices.Clone(element.Table.Value.Columns)
		for index := range clone.Table.Value.Columns {
			clone.Table.Value.Columns[index].ID = template.AllocateElementID(ids)
			ids.NextID++
		}
	}
	// Story 7.9 / D-7.7.10: a duplicate joins NO keep-together group.
	//
	// `clone := *element` above is a whole-struct copy, so without this line
	// the copy silently inherits the original's tag — and Epic 7 ships no way
	// anywhere in the designer to see a tag, set one or clear one (file-only
	// authoring is the stated scope boundary; FR51 asks only that a group can
	// be DECLARED). Duplicating a signature block would therefore enlarge a
	// keep-together set the author cannot reach, moving a page break for a
	// reason the product never shows them. The project refuses document state
	// the author cannot undo, and this is that rule at the copy path.
	//
	// It is cleared to the ZERO Presence, never to an explicit null: `Set:
	// true, Null: true` serializes back as `"keepTogether": null`, which is
	// still the key appearing in the file and still raises the document's
	// required format version. "No tag" is the field's absence. This is the
	// same spelling every other optional field's clear site uses
	// (`element.VisibleIf = template.Presence[string]{}`).
	clone.KeepTogether = template.Presence[string]{}
	width, height := projectedSize(clone)
	x, y := clone.X+6000, clone.Y+6000
	if snap {
		x, _ = snapToGrid(x)
		y, _ = snapToGrid(y)
	}
	if containComponent(projected, x, y, width, height) != nil {
		x, y = clone.X, clone.Y
	}
	clone.X, clone.Y = x, y
	return clone
}

// commandComponentIDs reads a selection's ids: non-empty, every entry a
// non-empty string, no entry twice.
func commandComponentIDs(raw map[string]json.RawMessage) ([]string, error) {
	var ids []string
	if json.Unmarshal(raw["ids"], &ids) != nil || len(ids) == 0 {
		return nil, componentFailure("", "component.ids", "command requires component ids")
	}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id == "" {
			return nil, componentFailure("", "component.ids", "component ids must be non-empty strings")
		}
		if seen[id] {
			return nil, componentFailure(id, "component.ids", "component ids must be unique")
		}
		seen[id] = true
	}
	return ids, nil
}

// workingComponentCopy and installComponentCopy bracket a multi-component
// command so that every id is resolved against, and every change is made to, a
// canonical clone; the caller's template is replaced only once the whole clone
// serializes, reparses and projects.
func workingComponentCopy(t *Template) (*Template, error) {
	before, err := SerializeTemplate(t)
	if err != nil {
		return nil, err
	}
	return ParseTemplate(before)
}

func installComponentCopy(t, working *Template) (designer.CanvasProjection, error) {
	canonical, err := SerializeTemplate(working)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	installed, err := ParseTemplate(canonical)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	projection, err := canvas(installed)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	t.doc, t.derivedFooters = installed.doc, installed.derivedFooters
	return projection, nil
}

// deleteComponents removes a whole selection in one command, so a group delete
// is one history entry. Every id is found before anything is removed.
func deleteComponents(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	if err := componentFields(raw, 3); err != nil {
		return designer.CanvasProjection{}, err
	}
	ids, err := commandComponentIDs(raw)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	working, err := workingComponentCopy(t)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	for _, id := range ids {
		band, _, index, _, err := findComponent(working, id)
		if err != nil {
			return designer.CanvasProjection{}, componentFailure(id, "component.id", "component was not found")
		}
		band.Elements = append(band.Elements[:index:index], band.Elements[index+1:]...)
	}
	return installComponentCopy(t, working)
}

// duplicateComponents copies a whole selection in one command. Each copy obeys
// duplicateComponent's rules, in its source's band, and every copy's ids come
// from one counter in the order the ids were given.
//
// SPEC-multi-pages: an optional `page` pastes every content copy onto that
// page. A copy landing on a page other than its source's keeps the source's
// page-local position (there is nothing there to stair-step away from); a copy
// on its own page is offset as always. Header and footer copies ignore it.
func duplicateComponents(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	fields := 4
	if _, ok := raw["page"]; ok {
		fields++
	}
	if err := componentFields(raw, fields); err != nil {
		return designer.CanvasProjection{}, err
	}
	ids, err := commandComponentIDs(raw)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	snap, err := commandBool(raw, "snap")
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	target, hasPage, err := optionalPageField(t, raw)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	working, err := workingComponentCopy(t)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	pageOf := contentPageIndex(working)
	type source struct {
		band      *template.Band
		projected designer.CanvasBand
		element   template.Element
		page      int
	}
	sources := make([]source, 0, len(ids))
	idsNeeded := int64(0)
	for _, id := range ids {
		band, projected, _, element, err := findComponent(working, id)
		if err != nil {
			return designer.CanvasProjection{}, componentFailure(id, "component.id", "component was not found")
		}
		// Copy the element value now: appending clones below may move the
		// band's backing array out from under a pointer.
		sources = append(sources, source{band: band, projected: projected, element: *element, page: pageOf[id]})
		idsNeeded += componentIDsNeeded(*element)
	}
	if working.doc.NextID <= 0 || working.doc.NextID > (1<<63-1)-idsNeeded {
		return designer.CanvasProjection{}, fmt.Errorf("folio8: nextId cannot allocate another component")
	}
	counter := template.Document{NextID: working.doc.NextID}
	for _, src := range sources {
		band, page := src.band, src.page
		clone := cloneComponent(src.element, src.projected, snap, &counter)
		if hasPage && src.projected.Name == bandContent {
			band = working.doc.ContentBands()[target]
			if target != page {
				clone.X, clone.Y = pastedPosition(src.element, src.projected, snap)
			}
			page = target
		}
		// The copy is judged by the break of the page it lands on.
		if err := refuseSectionBreakStraddleOnPage(working, src.projected.Name, page, clone, "component.geometry"); err != nil {
			return designer.CanvasProjection{}, err
		}
		band.Elements = append(band.Elements, clone)
	}
	working.doc.NextID = counter.NextID
	return installComponentCopy(t, working)
}

// pastedPosition is where a copy pasted onto another page lands: the source's
// own page-local position, snapped when that still fits the band.
func pastedPosition(element template.Element, projected designer.CanvasBand, snap bool) (geom.Length, geom.Length) {
	x, y := element.X, element.Y
	if !snap {
		return x, y
	}
	width, height := projectedSize(element)
	sx, _ := snapToGrid(x)
	sy, _ := snapToGrid(y)
	if containComponent(projected, sx, sy, width, height) != nil {
		return x, y
	}
	return sx, sy
}

func projectedSize(element template.Element) (geom.Length, geom.Length) {
	if element.Type != template.ElementTable {
		return element.Width.Value, element.Height.Value
	}
	if element.Width.Set {
		return element.Width.Value, element.Table.Value.HeaderHeight
	}
	var width geom.Length
	for _, column := range element.Table.Value.Columns {
		width += column.Width
	}
	return width, element.Table.Value.HeaderHeight
}

// Grid snapping is a convenience applied to the caller's number, not a second
// constraint placed on it: a rectangle that fitted its band before the grid
// rounded it must still fit afterwards. Callers below pull an edge back to the
// last grid line that fits, and only ever when the unsnapped rectangle already
// fitted — so snapping can never turn a legal drag into a refusal, and
// geometry the caller placed outside the band is still refused unchanged.
func containEdge(value, limit geom.Length) geom.Length {
	if value <= limit {
		return value
	}
	return floorToGrid(limit)
}

func floorToGrid(value geom.Length) geom.Length {
	if value <= 0 {
		return 0
	}
	return geom.Length(int64(value) / designer.GridIncrement * designer.GridIncrement)
}

// The three band identities, as page_setup.go mints them and as every command
// path compares them. Story 7.5 made band identity LOAD-BEARING for the first
// time — the content band's vertical cap lifts and the two repeating bands'
// does not — and a fifth inline spelling of a bare string is exactly how a
// distinction like that diverges without anything going red.
const (
	bandPageHeader = "pageHeader"
	bandContent    = "content"
	bandPageFooter = "pageFooter"
)

// bandsCappingVertically names the bands whose DECLARED HEIGHT bounds a
// component's vertical extent.
//
// The content band is absent by MEANING, not by omission. A page header and a
// page footer REPEAT on every page, so each is exactly one page tall and a
// component that left it would have nowhere to be. The content band is a
// COLUMN that pagination slices into page-height windows (internal/layout's
// Paginate), so it has no single height to be inside of: a component below
// the foot of page one is on page two, not outside the document.
//
// THE MIRROR. folio-designer/src/engine-protocol.ts declares this same list
// under this same name, and engine-bounds-mirror.test.ts reads BOTH files and
// asserts they agree. D-7.4.5, as widened by Story 7.5: any invariant
// duplicated across the Go/TypeScript boundary moves in ONE commit, with a
// test that reads both sides. Lifting this here alone would ship a story
// invisible in the running app — the browser's copy of this gate drops the
// whole snapshot, terminates the worker and blanks the canvas, with no
// element id and no attributable error.
var bandsCappingVertically = []string{bandPageHeader, bandPageFooter}

// containComponent is the ONE band-extent validation in the designer command
// path. It enforces two DIFFERENT KINDS of constraint, which used to share a
// single eight-disjunct expression and are separated here by what they MEAN:
//
//   - REPRESENTATIONAL, and therefore universal: a negative coordinate or
//     size is not geometry at all, in any band. This is also the only place
//     negativity is caught — lengthField admits values down to
//     -MaxCanvasMillipoints — so these terms are load-bearing.
//   - HORIZONTAL, and therefore universal: the column is unbounded
//     vertically, never horizontally. A band is as wide as the printable
//     page and nothing may hang off its side.
//   - BAND CAPACITY, and therefore only where a band HAS a capacity: see
//     bandsCappingVertically.
//
// Every surviving refusal keeps the same message, to the byte, because from
// the author's side they are one complaint: this component is not inside that
// band.
//
// The split keys on band.Name INSIDE this function and never at a call site.
// findComponent, bandByName and hitTestBand each range over all three names,
// so every one of the eleven callers can receive any of the three bands.
func containComponent(band designer.CanvasBand, x, y, width, height geom.Length) error {
	outside := x < 0 || y < 0 || width < 0 || height < 0 || x > geom.Length(band.Width) || width > geom.Length(band.Width)-x
	if !outside && slices.Contains(bandsCappingVertically, band.Name) {
		outside = y > geom.Length(band.Height) || height > geom.Length(band.Height)-y
	}
	if outside {
		return fmt.Errorf("folio8: component geometry must stay within %s", band.Name)
	}
	return nil
}

// containEdgeY is containEdge on the VERTICAL axis, and after Story 7.5 it is
// a pull-back only in the bands that cap vertically.
//
// It exists because the pre-clamps at its call sites are GATED on the
// unsnapped rectangle already fitting, so lifting the content band's cap does
// not neutralise them — it WIDENS the gate. A drag that is refused outright
// today starts passing the probe, and a containEdge left in place would then
// quietly pull its Y back to the foot of page one: "this component may live
// on page four" would become "this component snapped to the bottom of page
// one", with no refusal and no explanation.
func containEdgeY(band designer.CanvasBand, value, limit geom.Length) geom.Length {
	if !slices.Contains(bandsCappingVertically, band.Name) {
		return value
	}
	return containEdge(value, limit)
}

// ---------------------------------------------------------------------------
// STORY 12.1: THE HEIGHT OF A CAPPING BAND, AS A COMMAND.
//
// Band.Height had no writer anywhere in the product. The loader accepted it,
// the renderer and the canvas both consumed it, and nothing set it — so a
// letterhead's header was whatever the starter file declared and changing it
// meant hand-editing the file the designer had just saved.
//
// It is an ARM ON THIS DOOR rather than two more keys on ApplyPageSetupCommand
// because that door gates on `len(raw) != 7` and every caller's shape is that
// arity. The seven font-chain arms are the shipped precedent for a
// document-level mutation reached through the component door.

// bandHeightPath is the DataPath every band-height refusal carries. The command
// names a BAND rather than an element, so ElementID stays empty on every
// refusal except the strand — the one refusal that has an element to name.
//
// Bounded the way fontChainPath is bounded and for the same reason: `band`
// arrives as a free string on the wire, the host cuts DataPath at
// maxComponentDataPathBytes BY BYTES, and a path arriving split through the
// middle of a rune locates nothing.
func bandHeightPath(name string) string {
	if name == "" {
		return "bands"
	}
	return truncateAtRuneBoundary("bands."+name+".height", maxComponentDataPathBytes)
}

// bandSnapPath locates a malformed `snap` AT THE FIELD THAT IS ACTUALLY WRONG.
// Reporting it at bands.<band>.height — which is what a shared path would do —
// tells the author their height is bad when their boolean is, and sends them to
// correct the one field the door had already accepted (D-000.25). It is bounded
// the same way bandHeightPath is, and for the same reason: `band` arrives as a
// free string on the wire and the host cuts DataPath by BYTES.
func bandSnapPath(name string) string {
	if name == "" {
		return "bands"
	}
	return truncateAtRuneBoundary("bands."+name+".snap", maxComponentDataPathBytes)
}

// setBandHeight is the only writer of Band.Height outside the loader.
//
// IT VALIDATES BEFORE IT MUTATES, and the two things it validates are two
// different failures with two different audiences:
//
//   - THE CONTENT WINDOW. A header and a footer that between them eat the whole
//     printable column leave the document with nowhere for content to be. The
//     arithmetic is bandsLeaveContentWindow's, in page_setup.go, and it is
//     CALLED here rather than restated: Canvas asks the same predicate while
//     LOADING and phrases its own bare refusal, this asks it of a CANDIDATE
//     height while AUTHORING and owes a located one.
//   - THE STRAND. Shortening a capping band can leave a component that is
//     inside it today outside it afterwards. containComponent is the predicate
//     — reused, unchanged, one new call site — and it is asked of EVERY element
//     in the band, not of a selected one, because the author selected nothing.
//
// THE REFUSAL NAMES THE ACT, NOT THE OBJECT, which is why containComponent's
// own sentence is not borrowed. "component geometry must stay within pageHeader"
// answers a command the author did not send: they set a height, they did not
// move anything. The predicate is right and the words are not, and a new
// sentence costs nothing under AD-14 because command refusals are error strings
// routed by prefix rather than registry codes.
//
// The height is echoed back to the author IN THE LITERAL THEY SENT, unless
// SNAPPING moved it. It reached lengthField as JSON and left it as a decimal
// with at most three places, so what goes into the message is digits, one
// optional sign and one optional point. When `snap` is true the value the
// author sent is not the value that would be written, and a refusal naming the
// discarded one would send them to correct a number the engine never held — so
// the echo is re-spelled through template.FormatPoints, which IS appendPoints,
// rather than through a second spelling of it.
//
// AND EVERY DERIVED QUANTITY BESIDE IT IS SPELLED IN THE SAME UNITS. The author
// types POINTS into a box labelled "(pt)" and the engine's refusal echoes that
// literal back with "pt" on it; a bound or a reach quoted in millipoints beside
// it would be four digits wider than the number it is about, and the one
// quantity that tells the author what to type next would be the one they cannot
// type. template.FormatPoints is appendPoints — the format's own canonical
// exact-decimal spelling, the same one the file on disk uses — so a message and
// the document agree by construction rather than by care.
func setBandHeight(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	if err := componentFields(raw, 5); err != nil {
		return designer.CanvasProjection{}, err
	}
	name, err := commandString(raw, "band")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure("", "bands", "band must be a non-empty string")
	}
	// Only the members of bandsCappingVertically have a height to set. The
	// content band is refused HERE, which is what keeps `height` absent from it
	// in every serialized document: writeBand emits the key on Set alone, so a
	// content band that was ever written would carry one forever.
	if !slices.Contains(bandsCappingVertically, name) {
		if name == bandContent {
			return designer.CanvasProjection{}, componentFailure("", bandHeightPath(name), "the content band's height is derived from the page and the two bands that cap it, and cannot be set")
		}
		return designer.CanvasProjection{}, componentFailure("", bandHeightPath(name), "only pageHeader and pageFooter have a height a command may set")
	}
	proposed, err := lengthField(raw, "height")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure("", bandHeightPath(name), err.Error())
	}
	// SNAPPING IS THE ENGINE'S, AND IT HAPPENS FIRST (Story 12.5, R3). Every
	// other geometry command already carries `snap` and rounds here, through
	// the one SnapToGrid there is; a browser that rounded for itself would be
	// the first grid arithmetic in folio-designer and a fourth spelling of a
	// rule stated once. The canvas boundary drag is the caller that needs it —
	// the panel's typed box passes false, so an author who types 83 still gets
	// 83 and no document the panel writes moves a byte.
	snap, err := commandBool(raw, "snap")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure("", bandSnapPath(name), err.Error())
	}
	literal := string(raw["height"])
	// AND IT HAPPENS BEFORE EVERY CHECK BELOW, so a refusal names the number
	// that would actually have been written. Snapping after the checks would
	// let a height pass the content-window bound and then be rounded past it;
	// snapping before them and leaving `literal` at what the author sent would
	// print a sentence about a value the engine had already discarded. So the
	// echoed literal is RE-SPELLED here, and only here, in the format's own
	// canonical decimal — the same appendPoints the file on disk uses. An
	// unsnapped command still echoes the author's own bytes, untouched.
	if snap {
		snapped, valid := snapToGrid(proposed)
		if !valid {
			return designer.CanvasProjection{}, componentFailure("", bandHeightPath(name), fmt.Sprintf("a %s height of %spt overflows grid snapping", name, literal))
		}
		proposed = snapped
		literal = template.FormatPoints(proposed)
	}
	if proposed < 0 {
		return designer.CanvasProjection{}, componentFailure("", bandHeightPath(name), fmt.Sprintf("a %s height of %spt is negative", name, literal))
	}
	band, otherName := &t.doc.Bands.PageHeader, bandPageFooter
	if name == bandPageFooter {
		band, otherName = &t.doc.Bands.PageFooter, bandPageHeader
	}
	// The document as it stands, projected once. Everything the checks below
	// need is read off it rather than recomputed: the band's width, the other
	// band's height, and the printable column.
	//
	// THE COLUMN IS SUMMED, NOT SUBTRACTED. `pageHeight − marginTop −
	// marginBottom` is band placement, and AD-24 gives that to internal/layout
	// alone. WHAT THE ARCH GUARD ACTUALLY CHECKS IS NARROWER THAN THAT RULE, and
	// saying otherwise would be claiming an enforcement nobody has:
	// internal/bandcomposition_arch_test.go walks every non-method function in
	// this package and flags a subtraction one of whose operand identifiers is
	// one of seven hardcoded band-height NAMES (PageHeaderHeight,
	// PageFooterHeight, their lower-case forms, headerHeight, footerHeight,
	// usableHeight) or MarginTop/MarginBottom. It catches the derivation as it
	// is ordinarily written; arithmetic over differently named variables —
	// this function's own `innerH` and `other` among them — is outside its
	// reach. The summing below is a discipline the guard corroborates, not one
	// it imposes. The three bands PARTITION the printable column exactly
	// (internal/layout's BandOrigins: "the partition is exact"), so their
	// projected heights add up to it, and adding up what layout.ContentHeight
	// already derived is not a second derivation of it.
	projection, err := canvas(t)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	var innerH, other geom.Length
	var projected designer.CanvasBand
	found := false
	for _, existing := range projection.Bands {
		innerH += geom.Length(existing.Height)
		switch existing.Name {
		case name:
			projected, found = existing, true
		case otherName:
			other = geom.Length(existing.Height)
		}
	}
	// REFUSE LOUDLY RATHER THAN PASS SILENTLY. If the band this command names is
	// missing from the projection, `projected` is the zero CanvasBand and its
	// Name is "" — which is in no list at all, so containComponent below would
	// skip the vertical test for every element and the strand check would report
	// nothing while reading exactly like a check that passed. A vacuous guard is
	// worse than no guard, because it is quoted as evidence.
	//
	// It is UNREACHABLE as the code stands: Canvas builds Bands from the three
	// fixed names on every call, and TestSetBandHeightSeesEveryBandItMaySet
	// asserts that both settable names are always there. The guard is against a
	// future Canvas, not against a document, and it stays because the failure it
	// prevents is invisible.
	if !found {
		return designer.CanvasProjection{}, componentFailure("", bandHeightPath(name), fmt.Sprintf("the %s band is not in this document's projection, so a height for it cannot be checked", name))
	}
	header, footer := proposed, other
	if name == bandPageFooter {
		header, footer = other, proposed
	}
	if !bandsLeaveContentWindow(header, footer, innerH) {
		// THE BOUND IS THE PREDICATE'S OWN. bandContentWindowCeiling is what
		// bandsLeaveContentWindow accepts up to; naming anything else here —
		// `innerH-other`, say — would be a second spelling of the arithmetic
		// inside the very function whose job is to CALL it, and the only symptom
		// of the drift would be a refusal quoting a number the check rejects.
		return designer.CanvasProjection{}, componentFailure("", bandHeightPath(name), fmt.Sprintf("a %s height of %spt leaves no content band: %s takes %spt of the %spt between the page margins, so %s must be at most %spt", name, literal, otherName, template.FormatPoints(other), template.FormatPoints(innerH), name, template.FormatPoints(bandContentWindowCeiling(other, innerH))))
	}
	// The band as it WOULD BE, in the three fields containComponent reads:
	// Name selects the vertical cap, Width bounds the horizontal extent, Height
	// is the number under test. X and Y are deliberately absent rather than
	// copied — the page footer's origin moves with its height, and a candidate
	// carrying the old one would be a coordinate this command has not derived.
	candidate := designer.CanvasBand{Name: projected.Name, Width: projected.Width, Height: int64(proposed)}
	for _, element := range band.Elements {
		_, height := projectedSize(element)
		// ONLY THE VERTICAL AXIS IS THIS COMMAND'S BUSINESS. containComponent
		// enforces the horizontal bound too, and a height cannot change whether
		// a component hangs off the SIDE of its band — so asking the whole
		// predicate would make one horizontally overflowing element refuse every
		// band-height change there is, under a sentence naming a vertical reach
		// it never measured. That is a wrong message for a real defect somewhere
		// else, and it is not this command's to report.
		//
		// The predicate is still CALLED, not re-implemented: x=0 and width=0
		// satisfy every horizontal term of containComponent for any band width,
		// leaving exactly the vertical cap plus the representational y<0 and
		// height<0 terms. Weakening the vertical test is what would not be
		// allowed, and nothing here does.
		if containComponent(candidate, 0, element.Y, 0, height) != nil {
			return designer.CanvasProjection{}, componentFailure(string(element.ID), bandHeightPath(name), fmt.Sprintf("a %s height of %spt would leave %s outside the band: it reaches %spt", name, literal, element.ID, template.FormatPoints(element.Y+height)))
		}
	}
	// Atomic on ONE field: the previous Presence is held, the new one written,
	// and the projection is what decides whether it stands. Nothing else in the
	// document has been touched by the time this line runs, so a restored
	// Presence restores the document byte for byte.
	previous := band.Height
	band.Height = template.Presence[geom.Length]{Set: true, Value: proposed}
	// SPEC-table-rules review item 1: a taller band shrinks the content
	// window, and must not strand a table's minHeight.
	if err := refuseStrandedFloor(t, bandHeightPath(name)); err != nil {
		band.Height = previous
		return designer.CanvasProjection{}, err
	}
	// spec-section-break: a taller band must not leave the break at or below
	// the content band's bottom.
	if err := refuseSectionBreakBeyondContent(t); err != nil {
		band.Height = previous
		return designer.CanvasProjection{}, err
	}
	updated, err := canvas(t)
	if err != nil {
		band.Height = previous
		return designer.CanvasProjection{}, err
	}
	return updated, nil
}

// ---------------------------------------------------------------------------
// STORY 12.2: THE DOCUMENT'S LOCALE AND ITS UTC OFFSET, AS COMMANDS.
//
// Document.Locale and Document.UTCOffset had a loader, two consumers (render.go's
// two expr.NewFormatContext sites) and NO WRITER: every author who wanted Thai
// dates hand-edited the file the designer had just saved. These are the writers.
//
// TWO FIELDS, TWO ARMS, ONE FIELD EACH (Story 15.2a: a command names exactly
// what it changes). setBandHeight carries a `band` discriminator because a band
// is one field on three interchangeable structures; `locale` and `utcOffset`
// are two independent top-level fields with no shared shape and no shared
// validation, so a single arm would have to branch its DataPath and could
// refuse a good locale because of a bad offset. Two arms give each refusal a
// fixed DataPath and no discriminator to rotate. The seven font-chain arms are
// the shipped precedent for one document-level structure served by several
// narrowly-named commands.
//
// THERE IS NO BACKSTOP HERE, AND THAT IS THE ASYMMETRY WITH STORY 12.1.
// Canvas(t) reads neither Locale nor UTCOffset — all of its refusal sites are
// geometry, component or font-chain checks — so the trailing Canvas(t) call
// that caught a content-window violation on its own in 12.1 catches nothing at
// all here. THE ARM'S OWN VALIDATION IS THE ONLY REFUSAL before the wasm
// layer's reparse: a validation gap in this arm is not a locatedness
// regression, it is an unrefused write.

// setDocumentLocale is the only writer of Document.Locale outside the loader.
//
// It validates through template.IsLocale — the SAME predicate parse.go asks —
// rather than against a second copy of AD-12's four tags, and derives the
// legal-value list in its refusal from template.LocaleTags, exactly as
// updateComponentProperties derives its align refusal from StyleAlignTokens. A
// command stricter or looser than its loader is a document the engine can stamp
// and then refuse to reopen.
func setDocumentLocale(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	if err := componentFields(raw, 3); err != nil {
		return designer.CanvasProjection{}, err
	}
	// commandString refuses a missing key, a non-string and the empty string in
	// one call, and its plain error is re-phrased here so the refusal is LOCATED
	// on the field the author has to change.
	tag, err := commandString(raw, documentLocalePath)
	if err != nil {
		return designer.CanvasProjection{}, componentFailure("", documentLocalePath, "locale must be a non-empty string")
	}
	if !template.IsLocale(tag) {
		return designer.CanvasProjection{}, componentFailure("", documentLocalePath, fmt.Sprintf("locale must be one of %s (AD-12)", strings.Join(template.LocaleTags, ", ")))
	}
	// Atomic on ONE field: the previous value is held, the new one written, and
	// the projection decides whether it stands. Nothing else in the document has
	// been touched by the time this line runs, so a restored value restores the
	// document byte for byte. UTCOffset is not read and not written here.
	previous := t.doc.Locale
	t.doc.Locale = tag
	updated, err := canvas(t)
	if err != nil {
		t.doc.Locale = previous
		return designer.CanvasProjection{}, err
	}
	return updated, nil
}

// setDocumentUTCOffset is the only writer of Document.UTCOffset outside the
// loader.
//
// It validates through template.IsUTCOffset — the SAME predicate parse.go asks
// — and renders template.UTCOffsetSyntax rather than re-typing the ±HH:MM
// phrase, so the sentence the author reads and the pattern that refused them
// cannot drift apart.
//
// D-12.C IS WHY THIS IS A ONE-LINE REUSE RATHER THAN A RANGE CHECK OF ITS OWN.
// The loader's pattern used to admit `+99:99`, which internal/expr then refused
// at render, and the fork — command reuses a loose predicate, command is
// stricter than its loader, or the loader is repaired — was dissolved by
// repairing the loader. The command is therefore not stricter than the file
// door and not looser: it is the same predicate, and they agree by construction
// rather than by care.
func setDocumentUTCOffset(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	if err := componentFields(raw, 3); err != nil {
		return designer.CanvasProjection{}, err
	}
	offset, err := commandString(raw, documentUTCOffsetPath)
	if err != nil {
		return designer.CanvasProjection{}, componentFailure("", documentUTCOffsetPath, "utcOffset must be a non-empty string")
	}
	if !template.IsUTCOffset(offset) {
		return designer.CanvasProjection{}, componentFailure("", documentUTCOffsetPath, fmt.Sprintf("utcOffset must match %s", template.UTCOffsetSyntax))
	}
	// Atomic on ONE field, exactly as the locale arm is. Locale is not read and
	// not written here.
	previous := t.doc.UTCOffset
	t.doc.UTCOffset = offset
	updated, err := canvas(t)
	if err != nil {
		t.doc.UTCOffset = previous
		return designer.CanvasProjection{}, err
	}
	return updated, nil
}

// ---------------------------------------------------------------------------
// STORY 12.3: A TABLE'S HEADER AND ITS ALTERNATING ROWS, AS COMMANDS.
//
// Three kinds that write the three table properties the engine has always
// accepted, stored and rendered and that no command could reach: HeaderHeight,
// AltRowBackground and the HeaderStyle block. Until this story the only way to
// author any of them was to hand-edit the file the designer had just saved.
//
// WHY THREE TOP-LEVEL KINDS AND NOT THREE KEYS ON applyPropertyChanges.
// setComponentAsset's doc comment already rules the shape: anything the
// {op,value} grammar cannot express, or where CLEAR must stay inexpressible,
// becomes its own kind. `headerHeight` is exactly that — required, a plain
// geom.Length, never absent, so there is no clear arm for it to have — and
// applyPropertyChanges' 23 keys are a flat list that reaches element.Style and
// never element.Table, with no nesting a headerStyle block could occupy.
//
// THEY ROUTE THROUGH applyTableColumnCommand, like the seven column arms, so a
// header edit is serialize -> reparse -> apply -> serialize -> reparse ->
// project -> install: one atomic mutation, one undo entry, and a candidate that
// fails format validation never reaches the caller's template.
//
// CLEARING IS THE ZERO Presence, NEVER AN EXPLICIT NULL, and the spelling
// matters more than it looks. `Set:true, Null:true` serializes the key back as
// `"key": null` — which is still the key in the file: it changes the bytes,
// burns an undo entry and raises the document's required format version, and
// for altRowBackground the serializer has no null branch at all, so it would
// write `""` and the loader would then refuse the document it just wrote. The
// zero Presence removes the key. `op: "null"` is refused by all three arms.

// tableHeaderStyleFields is the closed set of headerStyle fields a command may
// author, in the order a refusal names them.
//
// TWELVE, AND THE ONE ABSENTEE IS A RULING (D-8.1.2's map, stated in full in
// Story 8.1's Design Notes). `padding` stays out of THIS closed set: D-12.4.1,
// as revised by the owner on 2026-09-13, lets the Table Editor author a TABLE's
// own `style.padding.left/right` — through updateComponentProperties'
// `paddingLeft`/`paddingRight`, not here — and `headerStyle.padding` authoring
// remains out of scope. The inspector still offers no padding rows.
//
// ⚠ IT WAS NINE UNTIL STORY 14.8, AND THIS PARAGRAPH IS THE RULING IT
// RETIRED — EDITED, NOT DELETED, so the history reads. It said: "`border` is
// deferred — the resolver arm exists but it is a nested block the cascade
// treats block-granularly, and it waits on Story 14.8's BORDERS section."
// The BORDERS section exists now, and the block-granularity is still true and
// is exactly what the panel has to disclose in words — it is not a reason to
// keep the field unauthorable.
//
// THE THREE NEW MEMBERS ARE FLAT DOTTED KEYS — `border.width`, `border.color`,
// `border.edges` — AND NOT ONE `border` MEMBER CARRYING AN OBJECT. Three
// grounds, recorded because the next person who adds a NESTED member to this
// closed set meets the identical trap:
//
//  1. THE PRODUCT ALREADY ANSWERED THIS QUESTION IN THIS FILE. An element's
//     nested `style.border` is authored from three flat keys —
//     `borderWidth`, `borderColor`, `borderEdges` in applyPropertyChanges —
//     with materialise-on-write and collapse-on-clear. A block on the wire
//     would be a second, contradictory answer to a settled question.
//
//  2. A COMPOSITE `value` CANNOT EXPRESS ONE-ATTRIBUTE-AT-A-TIME AUTHORING.
//     The command surface is {id, field, op, value}: one field, one op. A
//     block `set` writes the whole object, so changing a width would mean
//     re-transmitting the colour and the edges read back from the projection —
//     read-modify-write across an async boundary, and a value the author never
//     chose riding in a command they did think they were sending. Sending only
//     the attribute the author touched and a block on the wire are
//     incompatible; the panel sends only what was touched.
//
//  3. DOTTED, NOT camelCase, AND THE REASON IS MEASURED. `path` below is
//     "table.headerStyle." + field, so a dotted field name produces
//     `table.headerStyle.border.width` — a path the document actually has.
//     camelCase would produce `table.headerStyle.borderWidth`, a key no
//     document carries, which is DW-333's defect at the element level
//     (propertyPath returns the bare command key and the element path reports
//     `component.borderWidth`).
//
// COLLAPSE-ON-CLEAR IS THE THIRD LEG OF THE SAME MECHANISM, not an extra.
// Flat keys + materialise-on-write + collapse-on-clear is ONE mechanism with
// three parts; cleanupEmptyHeaderStyle carried only two of the three until this
// story because nothing could reach the third. Without it, clearing the last
// border attribute leaves `border: {}`, whose `Border.Set` keeps the
// empty-headerStyle check from ever firing and pins `headerStyle` alive in the
// file forever.
//
// ⚠ IT WAS SEVEN UNTIL STORY 11.2, AND THIS PARAGRAPH IS THE RULING IT
// RETIRED — EDITED, NOT DELETED, so the history reads. It said: "`bold` and
// `italic` have NOWHERE TO RESOLVE FROM — resolveHeaderStyle has no Bold arm
// and no Italic arm, so a header style declaring either would be stored,
// serialized, and read by nothing." That was true and is no longer:
// resolveHeaderStyle now cascades both, in the same `.Set && !.Null` spelling
// its siblings use, and the header row resolves the declared variant from its
// own chain (FR57, AC2). The tripwire fired exactly as it was written to.
var tableHeaderStyleFields = []string{"fontFamily", "fontSize", "lineSpacing", "background", "color", "valign", "align", "bold", "italic", "border.width", "border.color", "border.edges"}

// tableCommandTarget repeats the two-line gate all seven column arms share:
// the element must exist and it must be a table. It is the same pair of
// sentences, from one place rather than ten.
func tableCommandTarget(t *Template, id string) (designer.CanvasBand, *template.Element, error) {
	_, band, _, element, err := findComponent(t, id)
	if err != nil {
		return designer.CanvasBand{}, nil, componentFailure(id, "table.id", "table was not found")
	}
	if element.Type != template.ElementTable || !element.Table.Set || element.Table.Null {
		return designer.CanvasBand{}, nil, componentFailure(id, "table.id", "component is not a table")
	}
	return band, element, nil
}

// tableCommandOp reads the {op[,value]} grammar at the TOP LEVEL of a command,
// where applyPropertyChanges reads it inside a per-key object. The rules are
// the same rules: `clear` carries no value, `set` carries exactly one, and the
// arity is counted so a surplus key is refused rather than ignored.
//
// `null` IS REFUSED HERE, ONCE, FOR ALL THREE ARMS. It is not offered for any
// field in this story: an explicit null is a key in the file, and every field
// these arms write either must be absent or must not exist at all.
func tableCommandOp(raw map[string]json.RawMessage, id, path string, base int) (string, json.RawMessage, error) {
	op, err := commandString(raw, "op")
	if err != nil {
		return "", nil, componentFailure(id, path, "op must be set or clear")
	}
	switch op {
	case "set":
		if err := componentFields(raw, base+1); err != nil {
			return "", nil, componentFailure(id, path, "a set operation requires exactly one value")
		}
		value, ok := raw["value"]
		if !ok {
			return "", nil, componentFailure(id, path, "a set operation requires exactly one value")
		}
		return op, value, nil
	case "clear":
		if err := componentFields(raw, base); err != nil {
			return "", nil, componentFailure(id, path, "a clear operation cannot carry a value")
		}
		return op, nil, nil
	case "null":
		return "", nil, componentFailure(id, path, "null is not an operation this field accepts; clear removes it")
	default:
		return "", nil, componentFailure(id, path, "op must be set or clear")
	}
}

// setTableHeaderHeight is the only writer of TableExt.HeaderHeight outside the
// loader and createComponentInBand's seed.
//
// IT OFFERS NO CLEAR, and that is the format speaking rather than a preference:
// parse_bands.go pre-seeds `headerHeight` into `consumed` and hard-errors on a
// missing key ("missing required field for a table"), and serialize.go emits it
// unconditionally. A cleared header height is a document that cannot be
// reopened.
//
// IT RE-CHECKS CONTAINMENT, exactly as updateTableColumn's width case does and
// for the same reason: projectedSize derives a TABLE's height from
// HeaderHeight — that IS the table's projected height — so growing it can push
// the table out of a band that caps vertically. Without this check an author
// could grow a table out of its page header.
//
// THE TABLE GATE IS ASKED FIRST, before the arity and before the op refusal,
// and all three arms agree on that order (Finding P8): a refusal must not name
// a field on an element that does not exist, so `{"id":"nope","op":"bogus"}` is
// "table was not found" on table.id rather than a sentence about headerHeight.
//
// AND THE ARITY REFUSAL IS LOCATED (Finding P7). componentFields returns a bare
// fmt.Errorf; returning it unwrapped made this the ONE refusal in the whole
// change that reached the author as an unlocated ENGINE_REJECTED with no field
// to look at. Every other refusal across the three arms goes through
// componentFailure, and now so does this one.
func setTableHeaderHeight(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	id, err := commandString(raw, "id")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure("", "table.id", err.Error())
	}
	band, element, err := tableCommandTarget(t, id)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	if _, refused := raw["op"]; refused {
		return designer.CanvasProjection{}, componentFailure(id, "table.headerHeight", "headerHeight is required: it accepts neither a clear nor a null")
	}
	if err := componentFields(raw, 4); err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "table.headerHeight", "setTableHeaderHeight takes exactly kind, version, id and height")
	}
	height, err := propertyLength(raw["height"], "height")
	if err != nil || height <= 0 {
		return designer.CanvasProjection{}, componentFailure(id, "table.headerHeight", "headerHeight must be a positive length")
	}
	element.Table.Value.HeaderHeight = height
	width, projected := projectedSize(*element)
	if err := containComponent(band, element.X, element.Y, width, projected); err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "table.headerHeight", err.Error())
	}
	if err := refuseSectionBreakStraddle(t, band.Name, *element, "table.headerHeight"); err != nil {
		return designer.CanvasProjection{}, err
	}
	return canvas(t)
}

// setTableMinHeight writes SPEC-table-rules §3's floor under the table's own
// box.
//
// IT OFFERS A CLEAR, and that is the whole difference from its sibling
// setTableHeaderHeight: `headerHeight` is REQUIRED by the format (a cleared one
// is a document that cannot be reopened), while `minHeight` is optional and
// "no floor" is a state an author must be able to get back to.
//
// A NON-POSITIVE FLOOR IS REFUSED HERE FOR THE LOADER'S REASON, restated rather
// than re-derived: `max(0, content)` is `content`, which is what omitting the
// key already means, so a zero floor is a second spelling of absence and the
// format has one.
//
// NO CONTAINMENT RE-CHECK, and that asymmetry with setTableHeaderHeight is
// deliberate. projectedSize derives a table's PROJECTED height from
// HeaderHeight alone — the canvas has no data, so it has no rows — and a floor
// is not the header. The floor's own placement rule is the one ParseTemplate
// enforces (a minHeight taller than the content window is
// TABLE_MIN_HEIGHT_UNPLACEABLE), and it is enforced at the file door where it
// belongs rather than approximated here against a band.
func setTableMinHeight(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	id, err := commandString(raw, "id")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure("", "table.id", err.Error())
	}
	band, element, err := tableCommandTarget(t, id)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	op, value, err := tableCommandOp(raw, id, "table.minHeight", 4)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	if op == "clear" {
		element.Table.Value.MinHeight = template.Presence[geom.Length]{}
		return canvas(t)
	}
	height, err := propertyLength(value, "value")
	if err != nil || height <= 0 {
		return designer.CanvasProjection{}, componentFailure(id, "table.minHeight", "minHeight must be a positive length: it is a floor under the table's derived height, and a floor of zero is what clearing it already means")
	}
	// Review item 2: a floor taller than the content window is refused HERE,
	// at its own field, rather than by the re-parse as an unlocated error.
	if band.Name == bandContent {
		if g, gerr := pageGeometryOf(t); gerr == nil {
			if window := layout.ContentHeight(g); height > window {
				return designer.CanvasProjection{}, componentFailure(id, "table.minHeight", fmt.Sprintf("minHeight %spt is taller than the content window (%spt), so the table would fit on no page", template.FormatPoints(height), template.FormatPoints(window)))
			}
		}
	}
	element.Table.Value.MinHeight = template.Presence[geom.Length]{Set: true, Value: height}
	return canvas(t)
}

// updateTableRules writes ONE attribute of SPEC-table-rules §2's interior-line
// block, on updateTableHeaderStyle's exact shape: one field per command, each
// validated by THE SAME PREDICATE THE LOADER ASKS, and a clear of the last
// attribute collapsing the block away.
//
// A CLEAR OF `between` REMOVES THE WHOLE BLOCK. A `rules` block with no
// boundary draws nothing at all, yet it still raises the document's version,
// so an author unticking the last boundary (the editor sends exactly this
// clear) is saying "no rules" — and a saved width or colour left behind would
// be a dead block in the file. Clearing `width` or `color` removes only that
// attribute, and the block goes away when nothing is left in it.
//
// A document holding an explicit `"rules": null` is left untouched by any
// clear: there is nothing to remove, and rewriting the null would change the
// bytes of a document the author did not edit.
//
// THE BOUNDARY SET IS template.IsRuleBoundary's, never a second literal pair
// here: a command door that admitted a boundary the file door refuses could
// stamp out a document the designer cannot reopen — the failure updateTableHeaderStyle's
// own comment names.
func updateTableRules(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	id, err := commandString(raw, "id")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure("", "table.id", err.Error())
	}
	_, element, err := tableCommandTarget(t, id)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	field, err := commandString(raw, "field")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "table.rules", err.Error())
	}
	switch field {
	case "width", "color", "between":
	default:
		return designer.CanvasProjection{}, componentFailure(id, "table.rules", "field must be width, color or between")
	}
	op, value, err := tableCommandOp(raw, id, "table.rules."+field, 5)
	if err != nil {
		return designer.CanvasProjection{}, err
	}

	rules := template.TableRules{}
	if element.Table.Value.Rules.Set && !element.Table.Value.Rules.Null {
		rules = element.Table.Value.Rules.Value
	}

	if op == "clear" {
		if element.Table.Value.Rules.Set && element.Table.Value.Rules.Null {
			return canvas(t)
		}
		switch field {
		case "width":
			rules.Width = template.Presence[geom.Length]{}
		case "color":
			rules.Color = template.Presence[string]{}
		case "between":
			element.Table.Value.Rules = template.Presence[template.TableRules]{}
			return canvas(t)
		}
		// THE BLOCK COLLAPSES WHEN NOTHING IS LEFT IN IT, the same move
		// cleanupEmptyStyle makes for element.Style: an empty `rules: {}`
		// is a key in the file that means nothing, and leaving one behind
		// would make "clear the last attribute" a no-op in the bytes.
		if !rules.Width.Set && !rules.Color.Set && !rules.Between.Set && len(rules.Extra) == 0 {
			element.Table.Value.Rules = template.Presence[template.TableRules]{}
			return canvas(t)
		}
		element.Table.Value.Rules = template.Presence[template.TableRules]{Set: true, Value: rules}
		return canvas(t)
	}

	switch field {
	case "width":
		width, werr := propertyLength(value, "value")
		if werr != nil || width < 0 {
			return designer.CanvasProjection{}, componentFailure(id, "table.rules.width", "rules.width must not be negative: a PDF line width is non-negative (ISO 32000-1 8.4.3.2); use 0 for the thinnest line")
		}
		rules.Width = template.Presence[geom.Length]{Set: true, Value: width}
	case "color":
		colour, cerr := propertyString(value)
		if cerr != nil || !validPropertyColor(colour) {
			return designer.CanvasProjection{}, componentFailure(id, "table.rules.color", "rules.color must be a #RRGGBB colour")
		}
		rules.Color = template.Presence[string]{Set: true, Value: colour}
	case "between":
		var names []string
		if jerr := json.Unmarshal(value, &names); jerr != nil {
			return designer.CanvasProjection{}, componentFailure(id, "table.rules.between", "between must be an array of boundary names")
		}
		seen := map[string]bool{}
		for _, name := range names {
			if !template.IsRuleBoundary(name) || seen[name] {
				return designer.CanvasProjection{}, componentFailure(id, "table.rules.between", "between must name each of columns, rows at most once")
			}
			seen[name] = true
		}
		// CANONICAL ORDER, written by the engine. The projection joins in
		// RuleBoundaryTokens' order and the browser's guard admits only
		// that order, so a command that stored the author's click order
		// would produce a document the guard then refuses to read back.
		ordered := make([]string, 0, len(names))
		for _, token := range template.RuleBoundaryTokens {
			if seen[token] {
				ordered = append(ordered, token)
			}
		}
		rules.Between = template.Presence[[]string]{Set: true, Value: ordered}
	}
	element.Table.Value.Rules = template.Presence[template.TableRules]{Set: true, Value: rules}
	return canvas(t)
}

// setTableAltRowBackground writes the one colour Story 4.8 already renders on
// odd zero-based collection indexes. This story adds NO rendering rule: the
// paint decision stays the six inline lines in collectBandTableRuns, untouched.
//
// THE COLOUR IS VALIDATED BY THE ENGINE'S OWN PREDICATE, template.IsHexColour via
// validPropertyColor — the same one the loader refuses every colour field by
// (and style.background and style.color with it). The panel invents no second validation and shows this
// sentence.
//
// THE TABLE GATE IS ASKED BEFORE THE OP GRAMMAR (Finding P8), the same order
// its two siblings now use.
func setTableAltRowBackground(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	id, err := commandString(raw, "id")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure("", "table.id", err.Error())
	}
	_, element, err := tableCommandTarget(t, id)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	op, value, err := tableCommandOp(raw, id, "table.altRowBackground", 4)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	if op == "clear" {
		element.Table.Value.AltRowBackground = template.Presence[string]{}
		return canvas(t)
	}
	colour, err := propertyString(value)
	if err != nil || !validPropertyColor(colour) {
		return designer.CanvasProjection{}, componentFailure(id, "table.altRowBackground", "altRowBackground must be a #RRGGBB colour")
	}
	element.Table.Value.AltRowBackground = template.Presence[string]{Set: true, Value: colour}
	return canvas(t)
}

// updateTableHeaderStyle writes ONE field of the header-only Style block, and
// it is the second non-test writer of any HeaderStyle field — renameFontChain,
// which rewrites HeaderStyle.FontFamily when a chain is renamed, is the first
// and was the only one before this story.
//
// EVERY FIELD VALIDATES THROUGH THE SAME PREDICATE THE LOADER ASKS, never a
// second literal beside it: template.IsTableStyleAlign for align (the table
// triple — `justify` is refused here exactly as the file door refuses it),
// template.IsStyleValign for valign, template.DecodeLineSpacingRaw for
// lineSpacing, knownFontFamily for fontFamily and validPropertyColor for the
// two colours. A command door that admitted what the file door refuses could
// stamp out a document the designer cannot reopen.
//
// CLEARING THE LAST FIELD REMOVES THE BLOCK, AND SINCE STORY 14.8 THAT IS TWO
// NESTED COLLAPSES RATHER THAN ONE. An empty `headerStyle: {}` is a key in the
// file that means nothing, so cleanupEmptyHeaderStyle drops it; and when the
// field just cleared was one of the three `border.*` attributes, an empty
// `border: {}` it just produced goes first — the same two moves
// cleanupEmptyStyle makes for element.Style. ⚠ The inner collapse is passed the
// cleared FIELD and fires only for a `border.*` clear, because unlike element
// `style.border` an empty header `border: {}` is a MEANINGFUL document that wins
// the cascade whole; see cleanupEmptyHeaderStyle's own comment.
//
// THE THREE BORDER ATTRIBUTES ARE WRITTEN ONE AT A TIME AND SEED NOTHING. The
// cascade takes the header's border WHOLE (see headerBorderFor below), so the
// first attribute authored is the moment the table's border stops reaching the
// header row — a consequence the PANEL discloses in words, never one this arm
// papers over by filling in the other two.
func updateTableHeaderStyle(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	id, err := commandString(raw, "id")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure("", "table.id", err.Error())
	}
	// THE TABLE GATE IS ASKED FIRST (Finding P8). Naming a headerStyle field on
	// an element that does not exist is a refusal that points the author at the
	// wrong thing; all three arms now reach this gate at the same point.
	_, element, err := tableCommandTarget(t, id)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	field, err := commandString(raw, "field")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure(id, "table.headerStyle", err.Error())
	}
	if !slices.Contains(tableHeaderStyleFields, field) {
		return designer.CanvasProjection{}, componentFailure(id, "table.headerStyle", "headerStyle field must be one of "+strings.Join(tableHeaderStyleFields, ", "))
	}
	path := "table.headerStyle." + field
	op, value, err := tableCommandOp(raw, id, path, 5)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	// CLEARING A FIELD OF A BLOCK THAT IS NOT THERE MUST MOVE NO BYTES (Finding
	// P9), and it is checked BEFORE headerStyleFor rather than inside it,
	// because a SET legitimately needs the block materialised. headerStyleFor
	// replaces an explicit `"headerStyle": null` with a real block, and
	// cleanupEmptyHeaderStyle then drops the whole key — null -> {} -> absent,
	// three states walked for a command asked to remove a field that was
	// ALREADY absent. The document would change and an undo entry would burn,
	// and the author would have got nothing for either. An absent or null block
	// has no field to clear, so this is a no-op, and folio-go/internal/wasm/engine.go's
	// canonical-bytes short-circuit then reports it as the silent success it is.
	if op == "clear" && (!element.Table.Value.HeaderStyle.Set || element.Table.Value.HeaderStyle.Null) {
		return canvas(t)
	}
	style := headerStyleFor(element)
	if op == "clear" {
		switch field {
		case "fontFamily":
			style.FontFamily = template.Presence[string]{}
		case "fontSize":
			style.FontSize = template.Presence[geom.Length]{}
		case "lineSpacing":
			style.LineSpacing = template.Presence[int64]{}
		case "background":
			style.Background = template.Presence[string]{}
		case "color":
			style.Color = template.Presence[string]{}
		case "valign":
			style.Valign = template.Presence[string]{}
		case "align":
			style.Align = template.Presence[string]{}
		case "bold":
			style.Bold = template.Presence[bool]{}
		case "italic":
			style.Italic = template.Presence[bool]{}
		// THE THREE BORDER ATTRIBUTES CLEAR WITHOUT MATERIALISING THE BLOCK,
		// which is applyPropertyChanges' own spelling for `borderEdges`: a
		// clear against a border that is not there has nothing to remove, and
		// creating an empty `border: {}` only to have cleanupEmptyHeaderStyle
		// drop it again walks the document through a state it should never hold.
		case "border.width":
			if style.Border.Set && !style.Border.Null {
				style.Border.Value.Width = template.Presence[geom.Length]{}
			}
		case "border.color":
			if style.Border.Set && !style.Border.Null {
				style.Border.Value.Color = template.Presence[string]{}
			}
		case "border.edges":
			if style.Border.Set && !style.Border.Null {
				style.Border.Value.Edges = template.Presence[[]string]{}
			}
		}
		cleanupEmptyHeaderStyle(element, field)
		return canvas(t)
	}
	switch field {
	case "fontSize":
		size, err := propertyLength(value, "fontSize")
		if err != nil || size <= 0 {
			return designer.CanvasProjection{}, componentFailure(id, path, "fontSize must be a positive length")
		}
		style.FontSize = template.Presence[geom.Length]{Set: true, Value: size}
	case "lineSpacing":
		thousandths, err := template.DecodeLineSpacingRaw(value)
		if err != nil {
			return designer.CanvasProjection{}, componentFailure(id, path, err.Error())
		}
		style.LineSpacing = template.Presence[int64]{Set: true, Value: thousandths}
	case "bold", "italic":
		// The ONLY booleans in the set, so they take their own arm above
		// the string default — read with the same propertyBool the
		// element-level style.bold command uses, never a second decoder.
		flag, err := propertyBool(value)
		if err != nil {
			return designer.CanvasProjection{}, componentFailure(id, path, field+": "+err.Error())
		}
		if field == "bold" {
			style.Bold = template.Presence[bool]{Set: true, Value: flag}
		} else {
			style.Italic = template.Presence[bool]{Set: true, Value: flag}
		}
	case "border.width":
		// A LENGTH, READ BY THE SAME DECODER EVERY OTHER LENGTH ON THIS PATH
		// USES, and then bounded by the rule the LOADER asks rather than a
		// second one invented here: parse_bands.go refuses a NEGATIVE border
		// width (ISO 32000-1 §8.4.3.2 — a PDF line width is non-negative) and
		// accepts ZERO, which is the thinnest device line PDF can draw and not
		// an absent border. The check is restated at this door only because it
		// has to be LOCATED: without it the refusal arrives from ParseTemplate
		// on the round-trip in wasm's Apply, which names no element and no
		// field, and the author is told nothing they can act on.
		width, err := propertyLength(value, "border.width")
		if err != nil {
			return designer.CanvasProjection{}, componentFailure(id, path, "border.width must be a length in points with at most three decimal places")
		}
		if width < 0 {
			return designer.CanvasProjection{}, componentFailure(id, path, "border.width must not be negative: a PDF line width is non-negative (ISO 32000-1 8.4.3.2); use 0 for the thinnest line")
		}
		headerBorderFor(style).Width = template.Presence[geom.Length]{Set: true, Value: width}
	case "border.edges":
		// AN ARRAY ON THE WIRE, decoded exactly as applyPropertyChanges decodes
		// `borderEdges`: a plain json.Unmarshal into []string, refusing the
		// EMPTY array. An empty edge list is not "no border" — it is a stroke
		// with no side to draw, which the author expresses by clearing the
		// attribute instead.
		//
		// AND EVERY NAME IS CHECKED AGAINST THE LOADER'S OWN CLOSED SET, for
		// the same reason the width and colour arms restate the loader's rules:
		// so the refusal is LOCATED. Without it `["middle"]` is admitted here,
		// mutates the document, and the refusal arrives from ParseTemplate on
		// wasm's round-trip naming no element and no field. The set is not a
		// second literal beside the loader's — template.IsBorderEdge reads the
		// very map parse_bands.go reads, so this door cannot legalise a value
		// the file door still refuses.
		var edges []string
		if json.Unmarshal(value, &edges) != nil || len(edges) == 0 {
			return designer.CanvasProjection{}, componentFailure(id, path, "border.edges must be a non-empty string array")
		}
		for _, edge := range edges {
			if !template.IsBorderEdge(edge) {
				return designer.CanvasProjection{}, componentFailure(id, path, "border.edges must name only "+strings.Join(template.BorderEdgeTokens, ", ")+": "+edge+" is not one of them")
			}
		}
		headerBorderFor(style).Edges = template.Presence[[]string]{Set: true, Value: edges}
	default:
		text, err := propertyString(value)
		if err != nil {
			return designer.CanvasProjection{}, componentFailure(id, path, err.Error())
		}
		if stringsContainsPlaceholder(text) {
			return designer.CanvasProjection{}, componentFailure(id, path, field+" must not contain a placeholder")
		}
		switch field {
		case "fontFamily":
			if !knownFontFamily(t, text) {
				return designer.CanvasProjection{}, componentFailure(id, path, "fontFamily must name a declared non-empty font chain")
			}
			style.FontFamily = template.Presence[string]{Set: true, Value: text}
		case "background":
			if !validPropertyColor(text) {
				return designer.CanvasProjection{}, componentFailure(id, path, "background must be a #RRGGBB colour")
			}
			style.Background = template.Presence[string]{Set: true, Value: text}
		case "color":
			if !validPropertyColor(text) {
				return designer.CanvasProjection{}, componentFailure(id, path, "color must be a #RRGGBB colour")
			}
			style.Color = template.Presence[string]{Set: true, Value: text}
		case "valign":
			if !template.IsStyleValign(text) {
				return designer.CanvasProjection{}, componentFailure(id, path, "valign must be one of "+strings.Join(template.StyleValignTokens, ", "))
			}
			style.Valign = template.Presence[string]{Set: true, Value: text}
		case "align":
			if !template.IsTableStyleAlign(text) {
				return designer.CanvasProjection{}, componentFailure(id, path, "align must be one of "+strings.Join(template.TableStyleAlignTokens, ", "))
			}
			style.Align = template.Presence[string]{Set: true, Value: text}
		case "border.color":
			// THE SAME validPropertyColor THE OTHER TWO COLOURS ASK, which is
			// the loader's own predicate (template.IsHexColour): a colour the
			// file door refuses is refused here, located at the author's edit.
			if !validPropertyColor(text) {
				return designer.CanvasProjection{}, componentFailure(id, path, "border.color must be a #RRGGBB colour")
			}
			headerBorderFor(style).Color = template.Presence[string]{Set: true, Value: text}
		}
	}
	return canvas(t)
}

// headerBorderFor is headerStyleFor's one-level-deeper twin: it materialises the
// optional `border` block so ONE of its attributes can be written into it.
//
// ⚠ MATERIALISING THE BLOCK IS EXACTLY WHERE THE CASCADE CHANGES HANDS, and
// that is a product fact rather than a storage detail. resolveHeaderStyle takes
// the header's border WHOLE — `case hasHeader && header.Border.Set &&
// !header.Border.Null` — with the table's own `style.border` only as a sibling
// `case`, never a field-by-field merge. So the first attribute written here
// stops the table's border contributing to the header row at all, and whatever
// the author does not set falls to the format's own defaults instead of to the
// table's. Nothing is seeded on their behalf to soften that: TableColumns
// projects the resolved trio so the panel can SAY what will be drawn, and the
// panel says the takeover in words. Seeding the other two would write values
// the author never chose into their document.
func headerBorderFor(style *template.Style) *template.Border {
	if !style.Border.Set || style.Border.Null {
		style.Border = template.Presence[template.Border]{Set: true}
	}
	return &style.Border.Value
}

// headerStyleFor is styleFor's header-only twin: it materialises the optional
// block so a field can be written into it, replacing an explicit null with a
// real block rather than writing through one.
func headerStyleFor(element *template.Element) *template.Style {
	table := &element.Table.Value
	if !table.HeaderStyle.Set || table.HeaderStyle.Null {
		table.HeaderStyle = template.Presence[template.Style]{Set: true}
	}
	return &table.HeaderStyle.Value
}

// cleanupEmptyHeaderStyle is cleanupEmptyStyle's header-only twin, and it is
// what makes "clear the last header-style field" leave NO headerStyle key
// rather than an empty object. Extra is consulted for the same reason
// cleanupEmptyStyle consults it: unknown keys ride opaquely through a load and
// a save, and dropping a block that still carries one would delete an author's
// data. The one field this story cannot author (padding) is checked too — a
// hand-authored block that still declares it is not empty.
//
// ⚠ THE EMPTY-`border` COLLAPSE IS STORY 14.8's, AND IT IS THE THIRD LEG OF
// THE MECHANISM RATHER THAN A TIDY-UP. This function counted `style.Border.Set`
// from the day it was written but had no way to make it FALSE again, because
// nothing could clear a border attribute. Now something can, and without the
// collapse the last clear leaves `border: {}` — a block that declares nothing,
// whose `Border.Set` is nonetheless true, so the whole-style check below can
// never fire and `headerStyle` is pinned alive in the file forever. It is
// cleanupEmptyStyle's own six lines (`:1485-1490`), mirrored, and `Extra` is
// consulted inside the border for the same reason it is consulted outside it.
//
// ⚠ AND IT IS GATED ON `clearedField`, WHICH cleanupEmptyStyle's twin IS NOT,
// BECAUSE `border: {}` IS A MEANINGFUL DOCUMENT HERE. resolveHeaderStyle's arm
// is `case hasHeader && header.Border.Set && !header.Border.Null` — it takes the
// header's border block WHOLE, so a present-but-EMPTY border wins the cascade
// and paints the resolved 0.5pt black on all four edges instead of the table's
// border. writeBorder emits `{}` for it, so it is a load/serialize fixed point:
// a hand-authored `"border": {}` is a rendered choice, not debris. Ungated, this
// collapse ran on every one of the twelve fields' clears, so a command about
// `background` silently deleted that border and changed the PDF. A collapse must
// only ever remove a block THIS command emptied — hence the prefix test. The
// outer whole-`headerStyle` collapse below stays ungated: it removes a block
// whose emptiness this command's own clear is what produced.
//
// ⚠ IT MIRRORS cleanupEmptyStyle's Border HALF AND NOT ITS Padding HALF, and
// that asymmetry is DELIBERATE rather than an omission. `cleanupEmptyStyle`
// collapses an empty Border AND an empty Padding because element `style.padding`
// is authorable; `headerStyle.padding` is NOT (D-12.4.1, revised 2026-09-13,
// admits only a table's own `style.padding.left/right`, from the Table Editor;
// header-row padding authoring stays out of scope), so no command can ever
// empty it and a padding
// collapse here would have no clear to run on. The consequence is worth stating
// plainly rather than leaving implied: a HAND-AUTHORED `headerStyle:
// {"padding": {}}` still pins `headerStyle` alive in the file forever, because
// `style.Padding.Set` is true and nothing in this repo can make it false. That
// is a document the panel cannot produce and cannot clean up. If padding ever
// becomes authorable, the Padding half of `cleanupEmptyStyle:1491-1496` is what
// this function then owes — gated on `clearedField` the same way.
func cleanupEmptyHeaderStyle(element *template.Element, clearedField string) {
	table := &element.Table.Value
	if !table.HeaderStyle.Set || table.HeaderStyle.Null {
		return
	}
	style := &table.HeaderStyle.Value
	if strings.HasPrefix(clearedField, "border.") && style.Border.Set && !style.Border.Null {
		border := style.Border.Value
		if !border.Color.Set && !border.Width.Set && !border.Edges.Set && len(border.Extra) == 0 {
			style.Border = template.Presence[template.Border]{}
		}
	}
	if !style.Align.Set && !style.Background.Set && !style.Bold.Set && !style.Color.Set && !style.Italic.Set && !style.Border.Set && !style.FontFamily.Set && !style.FontSize.Set && !style.LineSpacing.Set && !style.Padding.Set && !style.Valign.Set && len(style.Extra) == 0 {
		table.HeaderStyle = template.Presence[template.Style]{}
	}
}

// ---------------------------------------------------------------------------
// STORY 8.1: THE DOCUMENT'S FONT CHAINS, AS COMMANDS.
//
// Six kinds that write the one document-level map nothing could write before
// (template.Document.Fonts). They are modelled on setComponentAsset, the
// module's other command that writes a document-level map and repoints the
// elements naming it (D-5.13.1), and they inherit its guarantee the same way:
// applyFontChainCommand serializes, reparses and projects a CLONE, and the
// caller's template is installed only if all three succeed. A rename that
// rewrites a map key and four element references is therefore ONE mutation,
// which is what lets wasm.Engine.Apply push exactly one undo entry for it.

// maxCanvasFontChainEntries bounds ONE chain's entry list the way
// maxCanvasFontFamilies bounds the chain list itself: the projection now
// carries the entries, so an unbounded chain is an unbounded projected array.
// A document declaring a longer chain is refused a projection with a stated
// reason, never silently cut, and the commands refuse to build one.
const maxCanvasFontChainEntries = 64

// maxComponentFailureMessageBytes and maxComponentDataPathBytes are the widths
// the wasm host cuts a ComponentCommandError's Message and DataPath to
// (wasm/cmd/engine/main.go's bounded(componentErr.Message, 512) and
// bounded(componentErr.DataPath, 256)). They are read here so a long id list
// and a long chain name are trimmed HERE — on a whole-id and a whole-rune
// boundary — instead of arriving at the author cut through the middle of an
// element id or a multi-byte character.
//
// They are HAND-COPIED, which is the one-sided-constant defect in pure form:
// wasm/cmd/engine is //go:build js && wasm, so `go test ./...` never compiles
// it and nothing links these two numbers to the host's literals. So they are
// tied by a source-reading tripwire instead —
// TestComponentFailureBoundsMatchTheHostsOwnLiterals reads main.go the way
// canvas_projection_wire_test.go reads engine-protocol.ts. Change one, change
// the other, in the same commit.
const (
	maxComponentFailureMessageBytes = 512
	maxComponentDataPathBytes       = 256
)

// fontChainPath is the DataPath every font-chain refusal carries. A chain
// command is not addressed to an element, so ElementID stays empty and the
// path names the map entry — the shape page-setup refusals already use, and
// the only one available: ComponentCommandError.ElementID is single-valued and
// the orphaning-delete refusal names a LIST of ids, which is why that list
// lives in Message.
// It is bounded HERE at maxComponentDataPathBytes because the two bounds
// disagree: a chain name is legal up to maxCanvasPropertyString (512), so
// "fonts." + name overruns the host's 256-byte DataPath cut, and the host's
// bounded() slices by BYTES — which on a multi-byte name splits a UTF-8 rune.
// The over-long-name refusal is the one case where locating the name is the
// entire point, so it is the one case that must not arrive mangled.
func fontChainPath(name string) string {
	if name == "" {
		return "fonts"
	}
	return truncateAtRuneBoundary("fonts."+name, maxComponentDataPathBytes)
}

// truncateAtRuneBoundary cuts value to at most limit BYTES without splitting a
// UTF-8 rune. The host's own bounded() does not do this, so anything this
// module hands it that could exceed a wire bound is cut here first.
func truncateAtRuneBoundary(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(value[cut]) {
		cut--
	}
	return value[:cut]
}

// applyFontChainCommand is the transaction wrapper, identical in shape to
// applyTableColumnCommand: a handler may mutate its candidate freely, and the
// caller's document is replaced only after that candidate serializes,
// reparses and projects.
func applyFontChainCommand(t *Template, raw map[string]json.RawMessage, apply func(*Template, map[string]json.RawMessage) error) (designer.CanvasProjection, error) {
	before, err := SerializeTemplate(t)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	working, err := ParseTemplate(before)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	if err := apply(working, raw); err != nil {
		return designer.CanvasProjection{}, err
	}
	canonical, err := SerializeTemplate(working)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	installed, err := ParseTemplate(canonical)
	if err != nil {
		return designer.CanvasProjection{}, componentFailure("", "fonts", "font chains did not pass format validation")
	}
	projection, err := canvas(installed)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	t.doc, t.derivedFooters = installed.doc, installed.derivedFooters
	return projection, nil
}

// fontChainName reads the author's chain name and applies the two rules every
// chain command shares: it is a non-empty string (commandString's own refusal,
// reused rather than restated) and it fits the projection's identifier bound,
// maxCanvasPropertyString. The length is refused HERE, located at fonts.<name>,
// so the author sees which name is too long instead of canvasFontChains'
// unlocated bare error firing later in the same transaction.
func fontChainName(raw map[string]json.RawMessage, field string) (string, error) {
	name, err := commandString(raw, field)
	if err != nil {
		return "", componentFailure("", "fonts", err.Error())
	}
	if len(name) > maxCanvasPropertyString {
		return "", componentFailure("", fontChainPath(name), "font chain name exceeds the projection bound")
	}
	return name, nil
}

// declaredFontChain resolves a chain a command is about to edit. It asks
// whether the KEY is declared, deliberately NOT template.Fonts.Chain: a chain
// with no entries is not one style.fontFamily may name, but decodeFonts
// accepts one at load and it must stay deletable and fillable rather than
// become unreachable to every command at once.
func declaredFontChain(t *Template, raw map[string]json.RawMessage) (string, []template.FontChainEntry, error) {
	name, err := fontChainName(raw, "name")
	if err != nil {
		return "", nil, err
	}
	chain, ok := t.doc.Fonts[name]
	if !ok {
		return "", nil, componentFailure("", fontChainPath(name), fmt.Sprintf("no font chain named %q is declared", name))
	}
	return name, chain, nil
}

// fontChainFace reads one face name: non-empty, and bounded by the same
// identifier bound the chain name is, because the projection now carries the
// entries too. A face this build's FontSet does not ship is ACCEPTED — the
// format's standing tolerance (render.go's resolveRuneFace skips a chain
// member absent from the set rather than failing), and a chain naming a face
// an embedding story will supply later is a legal chain today.
func fontChainFace(raw map[string]json.RawMessage, name, field string) (string, error) {
	face, err := commandString(raw, field)
	if err != nil {
		return "", componentFailure("", fontChainPath(name), err.Error())
	}
	if len(face) > maxCanvasPropertyString {
		return "", componentFailure("", fontChainPath(name), "font chain entry exceeds the projection bound")
	}
	return face, nil
}

func fontChainIndex(raw map[string]json.RawMessage, name, field string, limit int) (int, error) {
	index, err := commandInt(raw, field)
	if err != nil {
		return 0, componentFailure("", fontChainPath(name), err.Error())
	}
	if index < 0 || index > limit {
		return 0, componentFailure("", fontChainPath(name), "entry index is out of range")
	}
	return index, nil
}

// addFontChain declares a new chain. The duplicate-name refusal is this
// story's, not Story 8.2's: 8.2's panel only reports what the engine answers.
func addFontChain(t *Template, raw map[string]json.RawMessage) error {
	if err := componentFields(raw, 4); err != nil {
		return err
	}
	name, err := fontChainName(raw, "name")
	if err != nil {
		return err
	}
	if _, exists := t.doc.Fonts[name]; exists {
		return componentFailure("", fontChainPath(name), fmt.Sprintf("a font chain named %q already exists", name))
	}
	entriesRaw, ok := raw["entries"]
	if !ok {
		return componentFailure("", fontChainPath(name), "font chain entries are required")
	}
	entries, err := commandFontChainEntries(entriesRaw, name, "font chain entries")
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return componentFailure("", fontChainPath(name), "a font chain must declare at least one entry")
	}
	if len(entries) > maxCanvasFontChainEntries {
		return componentFailure("", fontChainPath(name), "a font chain declares more entries than the projection bound")
	}
	if len(t.doc.Fonts)+1 > maxCanvasFontFamilies {
		return componentFailure("", fontChainPath(name), "document declares more font chains than the projection bound")
	}
	if t.doc.Fonts == nil {
		t.doc.Fonts = template.Fonts{}
	}
	// Every entry this command can express is a FACE NAME. Story 8.6's
	// pick-and-embed command is what expresses an embedded entry; there is
	// deliberately no spelling in the command vocabulary that produces
	// anything else, and since Story 11.4 that is enforced by
	// commandFontChainEntries refusing `asset` rather than by the wire type
	// having nowhere to put one.
	t.doc.Fonts[name] = entries
	return nil
}

// renameFontChain moves the key AND carries every element that names it, in
// this one handler. fontFamily has exactly two attachment points in the model
// (see fontChainReferences), and a rename that moved only the key would leave
// them naming a chain that no longer exists — a document that loads and then
// fails at render. Because both halves happen inside one applyFontChainCommand
// transaction, wasm.Engine.Apply's single pushUndo covers the map and the
// elements together, and one undo restores all of them.
func renameFontChain(t *Template, raw map[string]json.RawMessage) error {
	if err := componentFields(raw, 4); err != nil {
		return err
	}
	name, chain, err := declaredFontChain(t, raw)
	if err != nil {
		return err
	}
	to, err := fontChainName(raw, "to")
	if err != nil {
		return err
	}
	if _, exists := t.doc.Fonts[to]; exists {
		// The destination is never silently destroyed, and renaming a chain
		// onto its own name is the same refusal: the key is taken.
		return componentFailure("", fontChainPath(to), fmt.Sprintf("a font chain named %q already exists", to))
	}
	delete(t.doc.Fonts, name)
	t.doc.Fonts[to] = chain
	for _, elements := range fontChainBands(t) {
		for i := range elements {
			element := &elements[i]
			if fontChainNamedBy(element.Style, name) {
				element.Style.Value.FontFamily.Value = to
			}
			if element.Table.Set && !element.Table.Null && fontChainNamedBy(element.Table.Value.HeaderStyle, name) {
				element.Table.Value.HeaderStyle.Value.FontFamily.Value = to
			}
		}
	}
	return nil
}

// deleteFontChain removes a chain nothing names. AC2's principle — a chain is
// never deleted with the orphaned elements left to fail at render — reaches
// headerStyle.fontFamily as squarely as style.fontFamily, so the refusal is
// measured over both.
func deleteFontChain(t *Template, raw map[string]json.RawMessage) error {
	if err := componentFields(raw, 3); err != nil {
		return err
	}
	name, chain, err := declaredFontChain(t, raw)
	if err != nil {
		return err
	}
	if referees := fontChainReferences(t, name); len(referees) > 0 {
		return componentFailure("", fontChainPath(name), fontChainOrphanMessage(name, referees))
	}
	delete(t.doc.Fonts, name)
	// Deleting a chain un-names every entry in it at once, which is the same
	// event removeFontChainEntry produces one at a time — so it collects the
	// same way, scoped to exactly those entries (AC5).
	dropUnnamedFontAssets(t, chain...)
	return nil
}

func addFontChainEntry(t *Template, raw map[string]json.RawMessage) error {
	if err := componentFields(raw, 5); err != nil {
		return err
	}
	name, chain, err := declaredFontChain(t, raw)
	if err != nil {
		return err
	}
	index, err := fontChainIndex(raw, name, "index", len(chain))
	if err != nil {
		return err
	}
	face, err := fontChainFace(raw, name, "face")
	if err != nil {
		return err
	}
	if len(chain)+1 > maxCanvasFontChainEntries {
		return componentFailure("", fontChainPath(name), "a font chain declares more entries than the projection bound")
	}
	t.doc.Fonts[name] = slices.Insert(slices.Clone(chain), index, template.FaceEntry(face))
	return nil
}

func moveFontChainEntry(t *Template, raw map[string]json.RawMessage) error {
	if err := componentFields(raw, 5); err != nil {
		return err
	}
	name, chain, err := declaredFontChain(t, raw)
	if err != nil {
		return err
	}
	from, err := fontChainIndex(raw, name, "from", len(chain)-1)
	if err != nil {
		return err
	}
	to, err := fontChainIndex(raw, name, "to", len(chain)-1)
	if err != nil {
		return err
	}
	moved := slices.Clone(chain)
	face := moved[from]
	moved = slices.Insert(slices.Delete(moved, from, from+1), to, face)
	t.doc.Fonts[name] = moved
	return nil
}

// removeFontChainEntry refuses to empty a chain. A chain with no entries is
// not one style.fontFamily may name (template.Fonts.Chain), so emptying one
// through this command would orphan every element naming it just as surely as
// deleting it would — an ADDITIONAL guard at the command path, not a
// relocation of the render-time rule, which stays where it is.
func removeFontChainEntry(t *Template, raw map[string]json.RawMessage) error {
	if err := componentFields(raw, 4); err != nil {
		return err
	}
	name, chain, err := declaredFontChain(t, raw)
	if err != nil {
		return err
	}
	index, err := fontChainIndex(raw, name, "index", len(chain)-1)
	if err != nil {
		return err
	}
	if len(chain) == 1 {
		return componentFailure("", fontChainPath(name), fmt.Sprintf("removing that entry would leave font chain %q with no entries", name))
	}
	removed := chain[index]
	t.doc.Fonts[name] = slices.Delete(slices.Clone(chain), index, index+1)
	dropUnnamedFontAssets(t, removed)
	return nil
}

// dropUnnamedFontAssets is AC5: A FACE NOTHING NAMES ANY LONGER IS DROPPED BY
// THE COMMAND THAT UN-NAMED IT.
//
// WHY IT IS HERE AND NOT IN THE SERIALIZER. writeAssets (serialize.go) says so
// itself, and it is not a preference: AD-9 / D-1.4.3's P1 is
// `Parse(Serialize(d)) == d`, which FORCES the serializer to preserve an
// orphan unconditionally — a save that collected one would not be a fixed
// point, and a document may legally carry an asset nothing references
// (D-1.4.13, RP-11's positive control). Collecting orphans is a DESIGNER
// FEATURE: the author's own action removes the face, so the author's own
// command removes the bytes, and nothing happens behind their back on save.
// serialize.go is not touched by this story.
//
// SCOPED TO THE KEYS JUST UN-NAMED, NEVER A DOCUMENT-WIDE SWEEP. This is
// setComponentAsset's rule (D-5.13.3) applied to the other asset kind: the
// candidates are exactly the entries this command removed, and each is dropped
// only if assetKeyReferenced now answers false for it. An asset a SECOND chain
// still names is retained — that is the arm DW-80 would have got wrong, since
// the old walk answered false for every font asset in every document.
//
// It takes the removed ENTRIES rather than keys so a caller cannot pass a key
// it derived some other way; a face entry contributes no candidate at all,
// which Embedded() decides rather than the caller.
func dropUnnamedFontAssets(t *Template, removed ...template.FontChainEntry) {
	for _, entry := range removed {
		if !entry.Embedded() {
			continue
		}
		if assetKeyReferenced(t, entry.AssetKey) {
			continue
		}
		delete(t.doc.Assets, entry.AssetKey)
	}
}

// ---------------------------------------------------------------------------
// STORY 8.6: PICKING A FAMILY PUTS IT IN THE FILE.
//
// embedFontFamily is the command Story 8.5 left missing. Twenty-one catalogue
// typefaces reached the designer and NOTHING could author an embedded-face
// chain entry or write a font asset, so picking a family changed nothing about
// the document. addFontChainEntry only ever builds a FaceEntry, and `entries`
// arrives at addFontChain as a []string; there was no spelling in the command
// vocabulary that produced an AssetEntry.
//
// ONE COMMAND, ONE HISTORY ENTRY, ONE UNDO (AC1). Embedding the face and
// declaring the chain are one applyFontChainCommand transaction, which is what
// makes wasm.Engine.Apply's single pushUndo cover both: one undo takes the
// asset and the chain away together, and there is never a document that
// carries the bytes with nothing naming them or names a key the document does
// not carry.
//
// IT IS SHAPED ON setComponentAsset (D-5.13.1/D-5.13.3), the module's other
// asset-authoring command: a CLOSED payload the {op,value} property grammar
// cannot express, whose bytes GO alone decodes, bounds, hashes and admits —
// the browser hashes nothing and decides no legality. The one shape it adds is
// that this command also writes the RECORD, and refuses to embed a face whose
// caller cannot supply one.
//
// THE WRITER CAN NEVER PRODUCE A DOCUMENT ITS OWN PARSER WOULD REJECT. Story
// 8.6 made licence, licenceText and copyright REQUIRED of an asset a chain
// names (parse.go's requireEmbeddedFaceLicence). This command declares exactly
// such a chain, so it refuses the pick up front when any of the three is
// missing — with a located reason naming the field. Without that guard the
// refusal would still fire, but at the transaction's reparse, as the
// unlocated "font chains did not pass format validation": correct, and useless
// to whoever has to fix it.
func embedFontFamily(t *Template, raw map[string]json.RawMessage) error {
	if err := componentFields(raw, 12); err != nil {
		return err
	}
	name, err := fontChainName(raw, "name")
	if err != nil {
		return err
	}
	record, err := embeddedFontRecord(raw, name)
	if err != nil {
		return err
	}
	mediaType, err := commandString(raw, "mediaType")
	if err != nil {
		return componentFailure("", fontChainPath(name), err.Error())
	}
	decoded, err := embeddedFaceBytes(raw, name)
	if err != nil {
		return err
	}
	tail, err := embeddedFontTail(raw, name)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("%x", sha256.Sum256(decoded))
	// RECOGNITION AND STRUCTURE AT THE COMMAND, never relying on the loader as
	// the catcher — setComponentAsset's rule (AC1 there) and the same reason:
	// bytes this build cannot read are refused before anything is written to
	// t.doc.Assets. An unrecognised container is refused HERE even though the
	// FORMAT accepts one (D-1.8.1 as amended, mediaType is an open set): a
	// hand-written `.folio` may carry a face this build cannot draw, but a
	// pick the designer makes must never produce one.
	if ferr := template.DecodeFontForRender(mediaType, decoded, template.FontChainSite{AssetKey: key, ChainName: name}); ferr != nil {
		return componentFailure("", fontChainPath(name), ferr.Error())
	}
	// AND THE ONE CLASS THAT GATE STRUCTURALLY CANNOT SEE (Story 16.0,
	// D-16.6). DecodeFontForRender's fence is "can this build read these bytes
	// as a single face, and nothing more", and checkSfnt beneath it never
	// inspects a table TAG — so a VARIABLE face is readable as a single face
	// and sailed straight through, into a `.folio` that saved cleanly and then
	// failed at render, where fontset.New refuses it. About a quarter of what
	// Google publishes is a variable build, so this was reachable by an
	// ordinary pick.
	//
	// The refusal is THE RENDERER'S OWN, not a second sentence written here:
	// fontset.RefuseVariableFace is the single function fontset.New also calls,
	// and its message already names the fonttools varLib.instancer remedy the
	// author needs. The renderer's guard is KEPT — a hand-written `.folio`
	// bypasses this command entirely — so this is an addition, never a move.
	//
	// It sits beside DecodeFontForRender and BEFORE anything reaches
	// t.doc.Assets, which is setComponentAsset's stated rule applied to the one
	// class it currently misses: "bytes this build cannot read are refused
	// before anything is written to t.doc.Assets."
	if verr := fontset.RefuseVariableFace(name, decoded); verr != nil {
		return componentFailure("", fontChainPath(name), verr.Error())
	}
	// AND THE CLASS NEITHER OF THOSE TWO CAN SEE: bytes that CONTRADICT the
	// licence being claimed for them (Story 16.1b, D-16.R.5/D-16.R.7).
	//
	// Until Epic 16 the tie between a face's declared SPDX id and its own
	// `name` table was a BUILD-TIME test over the 21 reviewed catalogue faces
	// (font-catalogue.test.ts:355-366). Epic 16 lets a face arrive from the
	// published library at the moment of a pick, so that gate stops covering
	// the population — and a runtime check that did not carry the tie would
	// make Epic 16 STRICTLY WEAKER than what it replaces, on the exact axis
	// D-8.6.5 already cost this project once (17 of 21 faces under another
	// project's terms, green until review).
	//
	// IT IS SITED HERE AND NOT IN THE BROWSER because the browser is not the
	// only door: embedFontFamily is reachable from wasm.Engine.Apply and from
	// a hand-authored command, which is the same gap fontset.go's `fvar`
	// refusal exists to cover. One implementation also covers the local tier.
	//
	// AND IT IS THE `fontset` PACKAGE'S OWN JUDGEMENT, not a second one
	// written here — this file stays a CALLER, never a checker, for the same
	// single-authority reason the `fvar` refusal above is one function. The
	// declared id comes from the record about to be written beside these
	// bytes, and the guard re-reads the bytes rather than trusting any other
	// field of the same payload (D-16.R.7: both wire fields move together, so
	// a check over them proves nothing).
	//
	// Beside the two gates above and BEFORE anything reaches t.doc.Assets.
	if lerr := fontset.RefuseContradictedLicence(name, record.Licence.Value, decoded); lerr != nil {
		return componentFailure("", fontChainPath(name), lerr.Error())
	}

	// DEDUPE BY CONTENT HASH (AC2). If ANY chain already names this key the
	// pick is already in the document: no second asset, no second chain, and
	// — because the canonical bytes then do not move — no second history
	// entry either (wasm.Engine.Apply's no-op short-circuit). The existing
	// chain is what the author is offered.
	if assetKeyReferenced(t, key) {
		return nil
	}
	if _, exists := t.doc.Fonts[name]; exists {
		return componentFailure("", fontChainPath(name), fmt.Sprintf("a font chain named %q already exists", name))
	}
	if len(t.doc.Fonts)+1 > maxCanvasFontFamilies {
		return componentFailure("", fontChainPath(name), "document declares more font chains than the projection bound")
	}
	if len(tail)+1 > maxCanvasFontChainEntries {
		return componentFailure("", fontChainPath(name), "a font chain declares more entries than the projection bound")
	}

	if t.doc.Assets == nil {
		t.doc.Assets = map[string]template.Asset{}
	}
	// INSERTED ONLY IF ABSENT. Re-picking a family whose chain was deleted
	// re-declares the chain over the asset the document already carries
	// rather than storing a second copy of it — the key IS the content, so
	// "already there" and "identical bytes" are the same question.
	if _, exists := t.doc.Assets[key]; !exists {
		// Re-wrapped canonically at 76 columns (AD-9) by writeAssets on the
		// way out, whatever shape it is held in here.
		t.doc.Assets[key] = template.Asset{
			MediaType: mediaType,
			Data:      []string{base64.StdEncoding.EncodeToString(decoded)},
			Font:      template.Presence[template.FontRecord]{Set: true, Value: record},
		}
	}
	if t.doc.Fonts == nil {
		t.doc.Fonts = template.Fonts{}
	}
	// THE PICKED FACE FIRST, THE PROPOSED TAIL BEHIND IT (AC3). The tail is
	// the shipped faces for the scripts the picked face does not cover — since
	// Story 11.4 carrying the CUTS each of those faces has, so a document whose
	// Thai fallback is `Noto Sans Thai` can bold its Thai even though the
	// embedded face itself has no bold to declare. The author edits it with the
	// chain commands 8.1 already shipped, which is why nothing here is
	// privileged or locked.
	//
	// THE PICKED ENTRY DECLARES NO CUT, and that is not an omission: a pick
	// embeds ONE face, the catalogue is one upright static Regular per family,
	// and an entry may only declare a cut the document actually carries.
	entries := make([]template.FontChainEntry, 0, len(tail)+1)
	entries = append(entries, template.AssetEntry(key))
	entries = append(entries, tail...)
	t.doc.Fonts[name] = entries
	return nil
}

// embeddedFontRecord reads the six keys the document will record about the
// face. Three of them — licence, licenceText, copyright — are what parse.go
// REQUIRES of an asset a chain names; family, style and source are display and
// provenance and are required HERE for a different reason: this command is the
// designer's only door into the assets map, and a catalogue row that cannot
// say what the face is or where it came from is a row that should not ship a
// face into anybody's document.
func embeddedFontRecord(raw map[string]json.RawMessage, name string) (template.FontRecord, error) {
	var record template.FontRecord
	for _, field := range []struct {
		key string
		dst *template.Presence[string]
	}{
		{"family", &record.Family},
		{"style", &record.Style},
		{"licence", &record.Licence},
		{"licenceText", &record.LicenceText},
		{"copyright", &record.Copyright},
		{"source", &record.Source},
	} {
		value, err := commandString(raw, field.key)
		if err != nil {
			return template.FontRecord{}, componentFailure("", fontChainPath(name), err.Error())
		}
		// BLANK IS EMPTY HERE TOO, and it has to be: parse.go's
		// requireEmbeddedFaceLicence refuses whitespace-only terms, and a
		// command that admitted them would hand the transaction's reparse a
		// document its own parser rejects — a correct refusal, arriving as the
		// unlocated "font chains did not pass format validation". commandString
		// refuses "" and stops there, so the trim is this function's.
		if strings.TrimSpace(value) == "" {
			return template.FontRecord{}, componentFailure("", fontChainPath(name), "folio8: "+field.key+" must be a non-empty string")
		}
		// The licence TEXT is the one field with no small legal value, so it
		// is bounded by the payload rather than by the projection string
		// bound the others share — a real OFL is ~4 KB and the projection
		// bound is 512.
		if field.key != "licenceText" && len(value) > maxCanvasPropertyString {
			return template.FontRecord{}, componentFailure("", fontChainPath(name), "embedded face "+field.key+" exceeds the projection bound")
		}
		*field.dst = template.Presence[string]{Set: true, Value: value}
	}
	return record, nil
}

// embeddedFaceBytes decodes and bounds the face, exactly as setComponentAsset
// does for a picture and against the same host-memory bound (D-5.13.4): the
// payload arrives through one engine protocol and one 8 MiB ceiling, so one
// derived byte bound covers both asset kinds. The largest catalogue face is
// under half a megabyte, so the bound is nowhere near the pick; it is here
// because a command that writes arbitrary bytes into a document needs one.
func embeddedFaceBytes(raw map[string]json.RawMessage, name string) ([]byte, error) {
	dataRaw, ok := raw["data"]
	if !ok {
		return nil, componentFailure("", fontChainPath(name), "face data is required")
	}
	var dataB64 string
	if json.Unmarshal(dataRaw, &dataB64) != nil || dataB64 == "" {
		return nil, componentFailure("", fontChainPath(name), "face data must be a non-empty base64 string")
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(dataB64)
	if err != nil {
		return nil, componentFailure("", fontChainPath(name), "face data must be valid base64")
	}
	if len(decoded) == 0 {
		return nil, componentFailure("", fontChainPath(name), "face data cannot be empty")
	}
	if len(decoded) > maxComponentAssetBytes {
		return nil, componentFailure("", fontChainPath(name), fmt.Sprintf("face exceeds the %d-byte supported size", maxComponentAssetBytes))
	}
	return decoded, nil
}

// embeddedFontTail reads the proposed fallback tail: the shipped faces that
// follow the picked face in the chain.
//
// THE INVARIANT, UNCHANGED SINCE STORY 8.6: every entry it can express is a
// FACE NAME, and the ONE entry that names an asset is the one this command
// builds itself from the bytes it just hashed. A caller cannot put a second
// asset entry in a chain by writing one down.
//
// ⚠ ONLY THE MECHANISM MOVED (Story 11.4, route C). A `[]string` used to be
// what made that true; the wire shape is now the format's own chain entry —
// a bare face name, or an object naming a face and the cuts it has — and what
// makes it true is commandFontChainEntries REFUSING the `asset` discriminant.
// `[]string` was never the point: it could not express a tail entry's declared
// bold either, so a pick proposed `Noto Sans Thai` behind an embedded face and
// silently threw away the fact that the engine ships `Noto Sans Thai Bold`.
// This is the same mechanism-versus-property move D-11.2.2 made to admit the
// object form over the one-key rule.
//
// AN EMPTY TAIL IS LEGAL: a face that covers every script the document renders
// needs no fallback behind it, and a chain of one embedded entry is a chain
// (TestLoadNeitherResolvesNorRefusesAnEmbeddedEntry).
func embeddedFontTail(raw map[string]json.RawMessage, name string) ([]template.FontChainEntry, error) {
	tailRaw, ok := raw["tail"]
	if !ok {
		return nil, componentFailure("", fontChainPath(name), "the proposed fallback tail is required — write [] for a face that needs none")
	}
	return commandFontChainEntries(tailRaw, name, "the fallback tail")
}

// commandFontChainCuts is the CLOSED style-variant set a command may write,
// paired with the model field each key lands in — the command door's local
// projection of internal/template's fontChainVariants, which is the format's
// authority and is unexported.
//
// ⚠ IT IS THE ONLY SPELLING OF THE THREE KEYS ON THIS SIDE OF THE WALL. The
// key list this decoder validates against, the sentence that tells an author
// what they may write, and the fields the values land in are all derived from
// this one table, so a fourth cut cannot arrive in the grammar and be dropped
// by the decoder, or vice versa. TestTheCommandDoorsCutSetIsTheFormatsCutSet
// ties it to the format's own enumeration.
var commandFontChainCuts = []struct {
	key   string
	field func(*template.FontChainEntry) *string
}{
	{"bold", func(e *template.FontChainEntry) *string { return &e.Bold }},
	{"italic", func(e *template.FontChainEntry) *string { return &e.Italic }},
	{"boldItalic", func(e *template.FontChainEntry) *string { return &e.BoldItalic }},
}

// commandFontChainCutKeys projects the cut keys in their fixed order.
func commandFontChainCutKeys() []string {
	out := make([]string, 0, len(commandFontChainCuts))
	for _, cut := range commandFontChainCuts {
		out = append(out, cut.key)
	}
	return out
}

// commandFontChainEntryKeys is the whole key set an entry object may carry on
// the wire. `asset` is a MEMBER so that writing one is reported as the defect
// it is — "a command names a face, never an assets key" — rather than as an
// unrecognised key, which would send the author looking for a typo.
func commandFontChainEntryKeys() []string {
	return append([]string{"face", "asset"}, commandFontChainCutKeys()...)
}

// commandQuotedKeyList spells a key enumeration for a refusal, the way
// internal/template's quotedKeyList does for the loader's refusals, so no
// message on this side hand-writes a set the decoder no longer enforces.
func commandQuotedKeyList(keys []string) string {
	quoted := make([]string, len(keys))
	for i, key := range keys {
		quoted[i] = `"` + key + `"`
	}
	return strings.Join(quoted, ", ")
}

// fontChainEntryShape is what a COMMAND may write for one chain entry, spelled
// once and quoted by every refusal below so an author is always told what they
// MAY write and never only what they may not. It is folio-format.md's entry
// grammar with the `asset` arm removed — see embeddedFontTail for why that
// removal is the mechanism rather than a restriction.
//
// It is DERIVED from commandFontChainCuts rather than typed out, for the reason
// Story 8.3 gave: "unpinned wording in a refusal is wording that goes stale
// silently and sends the author to fix the one thing that was not wrong."
var fontChainEntryShape = `a face name (a string), or an object {"face": "<face name>"} carrying any of ` +
	commandQuotedKeyList(commandFontChainCutKeys()) +
	` naming the face it is drawn in at that weight and slope`

// commandFontChainEntries decodes a command's array of chain entries — the one
// decoder both `addFontChain`'s `entries` and `embedFontFamily`'s `tail` use,
// so the two doors cannot drift in what a pick may write.
//
// `subject` names the field in every array-level refusal ("font chain
// entries", "the fallback tail"), because a caller who wrote a malformed tail
// must not be sent to look at `entries`.
//
// WHAT IT DOES NOT CHECK, deliberately: whether the faces named exist in this
// build's FontSet. A chain naming a face the renderer was not given is a legal
// chain — the format's standing tolerance — and the same document is correct
// wherever that face IS supplied.
//
// WHAT IT DOES CHECK, and did not until Story 11.4's review: a variant naming
// its entry's own base. That refusal is the loader's (D-11.2.11) and the
// loader remains its authority — applyFontChainCommand reparses this decoder's
// output before installing it — but the reparse's refusal is the UNLOCATED
// "font chains did not pass format validation", which names neither the chain
// nor the key. Two doors that agree on the verdict may still disagree on what
// the author is told (TestASelfReferentialVariantThroughTheCOMMANDDoorIsRefused).
func commandFontChainEntries(raw json.RawMessage, name, subject string) ([]template.FontChainEntry, error) {
	refuse := func(reason string) error { return componentFailure("", fontChainPath(name), reason) }
	// A NULL ARRAY IS ITS OWN REFUSAL, for the reason a null cut is
	// (commandChainEntryString): encoding/json decodes null into a slice as a
	// NO-OP, so `"tail": null` arrived here as an EMPTY tail and was accepted —
	// a pick that meant to propose three fallback faces and mistyped the value
	// would have written a one-entry chain and been told nothing. Null is a
	// value; absence is a missing key, and embeddedFontTail already refuses
	// that one by name.
	if string(bytes.TrimSpace(raw)) == "null" {
		return nil, refuse(subject + " is present and null. Null is a VALUE, not an absence: write an array, and [] where it is meant to be empty")
	}
	var items []json.RawMessage
	if json.Unmarshal(raw, &items) != nil {
		return nil, refuse(subject + " must be an array of font chain entries")
	}
	entries := make([]template.FontChainEntry, 0, len(items))
	for _, item := range items {
		entry, err := commandFontChainEntry(item, refuse)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// commandFontChainEntry decodes ONE entry. The arm is chosen from the raw
// JSON's first non-space byte, the way the format's own decoder chooses it
// (internal/template/parse.go's decodeFontChainEntry), so a number, an array or
// a null is told what the legal shapes are instead of being reported as a
// failed object decode.
//
// ⚠ THE OBJECT ARM DECODES INTO A KEY MAP AND VALIDATES EXPLICITLY, AND IT MUST.
// A struct with `*string` fields and DisallowUnknownFields — what this decoder
// was until the shape was probed — cannot express three of the four rules
// below, and each hole was MEASURED through ApplyComponentCommand rather than
// reasoned about:
//
//	{"face":"Roboto","asset":null}          err=nil. A JSON null decodes to a
//	                                        nil pointer, so the key is PRESENT
//	                                        and reads as ABSENT — the `asset`
//	                                        refusal, the structural guarantee
//	                                        route C was chosen for, was
//	                                        bypassable by writing null.
//	{"face":"Roboto","bold":null}           err=nil, the cut silently dropped,
//	                                        while the LOADER refuses a null
//	                                        variant with a located load error.
//	                                        A command door and a load door
//	                                        disagreeing is how a document
//	                                        becomes unloadable by the product
//	                                        that wrote it.
//	{"FACE":"X"} / {"BOLDITALIC":"Y"}       err=nil. encoding/json matches field
//	                                        names CASE-INSENSITIVELY and
//	                                        DisallowUnknownFields cannot see it,
//	                                        so both were accepted where the
//	                                        loader (which reads a key map)
//	                                        refuses them.
//
// A REPEATED KEY IS THE ONE ARM THIS FUNCTION DOES NOT ANSWER, and deliberately:
// refuseDuplicateCommandKeys (:103) already answers it for the whole command,
// at every depth — it token-scans arrays and nested objects alike, so
// `entries:[{"face":"A","bold":"B","bold":"C"}]` is refused at the door, before
// any handler runs, naming the path `$.entries[0]`. Writing a second duplicate
// check here would be a second answer to a solved question and could disagree
// with the first. TestARepeatedKeyInsideAChainEntryIsRefusedAtTheDoor pins it.
//
// EVERY REFUSAL BELOW IS ITS OWN SENTENCE. An unrecognised key, a value of the
// wrong type, and a null are three different author mistakes, and one sentence
// covering all three tells an author only that something is wrong.
func commandFontChainEntry(item json.RawMessage, refuse func(string) error) (template.FontChainEntry, error) {
	trimmed := strings.TrimSpace(string(item))
	switch {
	case strings.HasPrefix(trimmed, `"`):
		var face string
		if json.Unmarshal(item, &face) != nil {
			return template.FontChainEntry{}, refuse("a font chain entry must be " + fontChainEntryShape)
		}
		if err := boundedChainFaceName(face, refuse); err != nil {
			return template.FontChainEntry{}, err
		}
		return template.FaceEntry(face), nil

	case strings.HasPrefix(trimmed, "{"):
		var obj map[string]json.RawMessage
		if json.Unmarshal(item, &obj) != nil {
			return template.FontChainEntry{}, refuse("a font chain entry object is malformed JSON. It is " + fontChainEntryShape)
		}
		// THE CLOSED SET, CHECKED CASE-SENSITIVELY AND IN SORTED ORDER so an
		// author with two unrecognised keys is always sent to the same one —
		// the discipline decodeFontChainEntry uses for the same reason.
		for _, key := range slices.Sorted(maps.Keys(obj)) {
			if !slices.Contains(commandFontChainEntryKeys(), key) {
				return template.FontChainEntry{}, refuse(`"` + key + `" is not a key a font chain entry may carry. The key set is CLOSED and it is CASE-SENSITIVE — ` +
					commandQuotedKeyList(commandFontChainEntryKeys()) + ` and nothing else, so "Bold" is not "bold". It is ` + fontChainEntryShape)
			}
		}
		// THE ASSET ARM, REFUSED BY NAME AND ON PRESENCE ALONE. The format
		// admits it; a command does not, because the ONE entry that may name an
		// asset is the one embedFontFamily builds itself from bytes it hashed.
		// ⚠ PRESENCE, NOT VALUE: `{"asset":null}` names the key, and a guard
		// that read the value would have let the whole guarantee through a null.
		if _, present := obj["asset"]; present {
			return template.FontChainEntry{}, refuse("a font chain entry written by a command names a FACE, never an assets key — the only entry that may name an asset is the one embedFontFamily builds from the bytes it was given. It is " + fontChainEntryShape)
		}
		// THE DISCRIMINANT IS REQUIRED, and its absence is a defect of its own
		// rather than "the face name is empty": an object carrying only variant
		// keys names no entry for them to decorate.
		faceRaw, present := obj["face"]
		if !present {
			return template.FontChainEntry{}, refuse("a font chain entry object must name the face it is: " + fontChainEntryShape)
		}
		face, err := commandChainEntryString("face", faceRaw, refuse)
		if err != nil {
			return template.FontChainEntry{}, err
		}
		if err := boundedChainFaceName(face, refuse); err != nil {
			return template.FontChainEntry{}, err
		}
		entry := template.FaceEntry(face)
		for _, cut := range commandFontChainCuts {
			// AN ABSENT KEY IS AN ABSENT CUT, and that is a first-class answer:
			// the engine reports the base face and a Warning. It is not the
			// same as a key present with nothing in it, which is why presence
			// is read from the map rather than from a decoded zero value.
			declaredRaw, present := obj[cut.key]
			if !present {
				continue
			}
			declared, err := commandChainEntryString(cut.key, declaredRaw, refuse)
			if err != nil {
				return template.FontChainEntry{}, err
			}
			if declared == "" {
				return template.FontChainEntry{}, refuse(`"` + cut.key + `" names the face this entry is drawn in at that weight and slope, and an empty string names none — for a cut this family does not have, write no key at all rather than an empty string`)
			}
			if len(declared) > maxCanvasPropertyString {
				return template.FontChainEntry{}, refuse("font chain entry exceeds the projection bound")
			}
			// THE SELF-REFERENCE, REFUSED HERE AND LOCATED (D-11.2.11).
			// ⚠ Only the BASE is privileged: two DIFFERENT cuts of one entry
			// may name the same face, and this comparison must never be
			// widened into "no two cuts may agree".
			//
			// The loader refuses this too, and until Story 11.4's review it was
			// the ONLY thing that did: applyFontChainCommand reparses what this
			// decoder built, so the author got the reparse's UNLOCATED "font
			// chains did not pass format validation" — a sentence naming
			// neither the chain, the entry, nor the key. The reparse is still
			// the backstop; this is the refusal an author can act on.
			if declared == face {
				return template.FontChainEntry{}, refuse(`"` + cut.key + `" names this entry's OWN base face, ` + strconv.Quote(face) +
					` — a cut names the face drawn INSTEAD of the base, so one naming the base declares no cut, and does it silently. Write no key at all, or name the face that IS the cut; two DIFFERENT cuts may agree, only the base is privileged (D-11.2.11)`)
			}
			*cut.field(&entry) = declared
		}
		return entry, nil

	default:
		return template.FontChainEntry{}, refuse("a font chain entry must be " + fontChainEntryShape)
	}
}

// commandChainEntryString reads one entry key's value, and it separates the two
// ways a value can fail to be a face name because they are two different author
// mistakes.
//
// ⚠ A NULL IS ITS OWN REFUSAL AND NOT "the wrong type". encoding/json decodes
// null into a string as a NO-OP — no error, the zero value — so a null read
// through an ordinary decode is indistinguishable from a key that was never
// written. That is exactly the confusion the format refuses to allow: an ABSENT
// key declares no cut, and the loader (decodeFontChainEntry) refuses a null
// with a located load error. If this door dropped it silently the two doors
// would disagree, and a document the product wrote would be one the product
// cannot reopen.
func commandChainEntryString(key string, raw json.RawMessage, refuse func(string) error) (string, error) {
	if string(bytes.TrimSpace(raw)) == "null" {
		return "", refuse(`"` + key + `" is present and null. Null is a VALUE, not an absence: to say this entry has no such cut write no key at all, and to declare one name the face. It is ` + fontChainEntryShape)
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return "", refuse(`"` + key + `" must be a string naming a face. It is ` + fontChainEntryShape)
	}
	return value, nil
}

// boundedChainFaceName is the two rules every face name a command writes obeys,
// in both arms: non-empty, and inside the projection's identifier bound. They
// are the rules addFontChain's `entries` and embedFontFamily's `tail` each
// carried separately before Story 11.4.
func boundedChainFaceName(face string, refuse func(string) error) error {
	if face == "" {
		return refuse("a font chain entry must be a non-empty string")
	}
	if len(face) > maxCanvasPropertyString {
		return refuse("font chain entry exceeds the projection bound")
	}
	return nil
}

// fontChainBands is the three top-level band element lists in DOCUMENT ORDER
// (pageHeader, content, pageFooter) — the order the orphaning-delete refusal
// names its ids in.
func fontChainBands(t *Template) [][]template.Element {
	// One slice per band, each sharing its band's storage, so renameFontChain
	// writes through to every designed page.
	bands := t.doc.ElementBands()
	out := make([][]template.Element, 0, len(bands))
	for _, band := range bands {
		out = append(out, band.Elements)
	}
	return out
}

func fontChainNamedBy(style template.Presence[template.Style], name string) bool {
	return style.Set && !style.Null && style.Value.FontFamily.Set && !style.Value.FontFamily.Null && style.Value.FontFamily.Value == name
}

// fontChainReferences reports the ids of every element naming name, in
// document order. It is the SAFETY half of a delete, like assetKeyReferenced:
// under-reporting a reference here deletes a chain something still names, with
// no compile error to announce it.
//
// MEASURED, NOT ASSUMED: fontFamily has exactly TWO attachment points in
// template's model — Element.Style.FontFamily and
// Element.Table.HeaderStyle.FontFamily (model.go's Style is reached from
// nowhere else), and both are live at render (render.go's fontChain resolves
// the first, table_render.go's header-style resolver the second). Columns,
// footers and assets carry no Style. If a later story attaches a Style
// anywhere else, this walk needs that location added: like assetKeyReferenced,
// findComponent and addCanvasImagePaint, it enumerates the three band lists by
// hand because the module has no shared element enumerator.
func fontChainReferences(t *Template, name string) []string {
	var ids []string
	for _, elements := range fontChainBands(t) {
		for _, element := range elements {
			if fontChainNamedBy(element.Style, name) || (element.Table.Set && !element.Table.Null && fontChainNamedBy(element.Table.Value.HeaderStyle, name)) {
				ids = append(ids, string(element.ID))
			}
		}
	}
	return ids
}

// fontChainOrphanListReserve is the room fontChainOrphanMessage keeps, past
// the chain name, for what the message still has to say. The shortest of those
// endings is the "%d elements" fallback, and 32 bytes covers that for any id
// count a document can hold — so reserving it makes the budget below positive
// for EVERY name fontChainName accepts, which is the whole point.
const fontChainOrphanListReserve = 32

// fontChainOrphanMessage names the blocking elements in the refusal itself,
// which is the only place a LIST of ids can go: ComponentCommandError carries
// one ElementID. The host cuts the message at maxComponentFailureMessageBytes,
// so a list that would not fit is trimmed here on a whole-id boundary and
// closed with " and N more" rather than left to be cut mid-id.
func fontChainOrphanMessage(name string, ids []string) string {
	// THE NAME IS TRIMMED FIRST, and without this the guarantee above is not
	// merely weakened but inverted: fontChainName admits a name up to
	// maxCanvasPropertyString (512), which alone is the whole message width,
	// so a long-named chain drives the budget below NEGATIVE, every branch
	// overruns, and the host cuts the message wherever it lands — through the
	// middle of the name, or of an id, or of a rune. The trim is measured
	// against the FORMATTED prefix rather than against len(name) because %q
	// escapes, so a name of quotes or backslashes widens by more than its own
	// length; it cuts on a rune boundary and marks the cut.
	label, elision := name, ""
	prefix := fmt.Sprintf("font chain %q is still named by ", label)
	for len(prefix) > maxComponentFailureMessageBytes-fontChainOrphanListReserve && label != "" {
		label, elision = truncateAtRuneBoundary(label, len(label)-1), "…"
		prefix = fmt.Sprintf("font chain %q is still named by ", label+elision)
	}
	budget := maxComponentFailureMessageBytes - len(prefix)
	more := func(remaining int) string { return fmt.Sprintf(" and %d more", remaining) }
	list := ""
	for i, id := range ids {
		candidate := id
		if i > 0 {
			candidate = list + ", " + id
		}
		// A trimmed list must fit WITH its suffix; a complete one needs none.
		need := len(candidate)
		if i+1 < len(ids) {
			need += len(more(len(ids) - i - 1))
		}
		if need > budget {
			if i == 0 {
				break
			}
			return prefix + list + more(len(ids)-i)
		}
		list = candidate
	}
	if list == "" {
		// Not even one id fits beside a name this long: the count is all that
		// is left to say, and it is still true.
		return prefix + fmt.Sprintf("%d elements", len(ids))
	}
	return prefix + list
}
