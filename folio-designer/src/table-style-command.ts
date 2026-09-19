// STORY 12.3. A table's header height, its alternating row background, and one
// field of its header style — as opaque Go-defined bytes.
//
// THREE KINDS, NOT ONE, because Go has three arms and not one: a command names
// exactly what it changes (Story 15.2a). They are also three top-level kinds
// rather than three keys threaded through `updateComponentProperties`, and that
// is `setComponentAsset`'s own shipped ruling rather than a preference —
// anything the `{op,value}` grammar cannot express, or where CLEAR must stay
// inexpressible, becomes its own kind. `headerHeight` is exactly that: the
// format requires it, so there is no clear for it to have, and NO CLEAR
// AFFORDANCE IS OFFERED ANYWHERE.
//
// THIS MODULE HOLDS NO RULE OF ITS OWN. It does not clamp, normalise, validate
// a colour, or decide what a legal font chain is. Those are engine rules — one
// predicate each, asked by the loader and the command door alike — and the
// panel renders the engine's own located sentence. What it does decide is
// TRANSPORT: which of the two JSON scalar types each field travels as, because
// Go decodes `fontSize` with a length decoder and `align` with a string one,
// and a value in the wrong JSON type could not reach either rule to be judged
// by it.
//
// DRAFTS TRAVEL AS TYPED. `jsonNumber` tests the author's literal against the
// JSON number grammar and sends it byte for byte or sends `null`; nothing here
// runs Number(). An emptied box therefore reaches Go as `null` (numeric) or
// `""` (string) and the engine names the field — never as a value nobody typed.
// That is band-height-command.ts's promise, in both the shapes it takes.
//
// CLEARING IS `op: "clear"`, WHICH REMOVES THE KEY. It is not `op: "null"` —
// no field in this story accepts that, and all three arms refuse it — because
// an explicit null is still the key in the file: it changes the bytes, burns an
// undo entry and raises the document's required format version.
import { commandBytes, jsonArray, jsonNumber, jsonString, type JsonField } from './command-json'

// The ten header-style fields THIS MODULE can author. The fields absent from
// this union are each a ruling, not an oversight: `padding` is forbidden
// outright by D-12.4.1, and `bold`/`italic` are held out on the transport
// ground stated below.
//
// STORY 14.8 ADDS THE HEADER BORDER, ONE ATTRIBUTE PER COMMAND. The names are
// the engine's own dotted spelling — `border.width`, `border.color`,
// `border.edges` — because `updateTableHeaderStyle` builds its located path as
// `table.headerStyle.` + field, so the dots make the refusal name a path the
// document actually has. There is deliberately NO `border` member carrying a
// whole object: the command surface is one field and one op, so a block `set`
// would have to re-transmit the two attributes the author did not touch, read
// back from the projection across an async boundary. Sending only what was
// touched and sending the block whole are incompatible.
//
// ⚠ `bold`/`italic` USED TO BE JUSTIFIED HERE AS "no arm in the engine's header
// cascade to resolve from — a header style declaring either would be stored and
// read by nothing". THAT SENTENCE IS RETIRED AT STORY 11.3 BECAUSE IT IS FALSE:
// Story 11.2 gave `resolveHeaderStyle` a bold and an italic arm, the engine's
// `tableHeaderStyleFields` is NINE, and Story 11.3 projects the committed and
// resolved pair for both (`headerBold`/`headerBoldResolved`,
// `headerItalic`/`headerItalicResolved`). A header style declaring either is
// stored, cascaded, drawn and now read back.
//
// THEY STAY OUT OF THIS UNION ON A DIFFERENT AND STILL-TRUE GROUND: their value
// is a BOOLEAN, and this factory encodes every value as a string or a number
// (`NUMERIC_HEADER_STYLE_FIELDS` below is the whole of its type knowledge), so
// it cannot build a command the engine would accept. There is also no control
// to send one — `TableEditor.tsx` has no header B/I — and DW-240 was read-back
// plumbing, not authoring. Adding either here means adding a boolean arm and
// the control that uses it, together.
export type TableHeaderStyleField = 'fontFamily' | 'fontSize' | 'lineSpacing' | 'background' | 'color' | 'valign' | 'align' | 'border.width' | 'border.color' | 'border.edges'

// The three header-style fields whose value is a NUMBER on the wire: two lengths
// in points and a dimensionless ratio. This is a transport fact about Go's
// decoders, not a second opinion about what a legal value is.
const NUMERIC_HEADER_STYLE_FIELDS: ReadonlyArray<TableHeaderStyleField> = ['fontSize', 'lineSpacing', 'border.width']

// The ONE header-style field whose value is a JSON ARRAY on the wire, because
// Go decodes it with a plain `json.Unmarshal` into `[]string` — the same decoder
// the element-level `borderEdges` command uses. The caller passes the edge names
// comma-joined, which is the spelling the PROJECTION uses for the same set, so
// the panel never has to convert between two shapes of one value.
//
// AN EMPTY DRAFT ENCODES AS `[]`, NOT AS A GUESS. Go refuses the empty array
// with a located sentence, exactly as it refuses one from the inspector's border
// control; sending `[""]` or silently dropping the command would each be this
// module inventing a rule, which it does not do. A panel that wants to remove
// the attribute sends `op: "clear"` instead.
const ARRAY_HEADER_STYLE_FIELDS: ReadonlyArray<TableHeaderStyleField> = ['border.edges']

// `height` is the author's DRAFT, in points, passed as typed. There is no
// clear: `headerHeight` is required by the format (`parse_bands.go` hard-errors
// on its absence and `serialize.go` emits it unconditionally), so a cleared one
// is a document that cannot be reopened. The engine refuses a clear op on it
// too — this factory simply cannot build one.
export function tableHeaderHeightCommand(id: string, height: string): ArrayBuffer {
  return commandBytes('setTableHeaderHeight', [['id', jsonString(id)], ['height', jsonNumber(height)]])
}

// The alternating row background: a `#RRGGBB` draft, or a clear that removes
// the key and returns those rows to `style.background`. Story 4.8 already
// renders it; this only writes it.
export function tableAltRowBackgroundCommand(id: string, operation: 'set' | 'clear', value = ''): ArrayBuffer {
  return commandBytes('setTableAltRowBackground', [['id', jsonString(id)], ...operationFields(operation, jsonString(value))])
}

// One field of the header-only Style block. A cleared field is removed from the
// block, and clearing the LAST one removes the block itself — both are the
// engine's doing, not this module's. Since Story 14.8 that collapse is two deep:
// clearing the last border attribute removes the empty `border` block, and if it
// was the block's only member the `headerStyle` key goes with it. Again the
// engine's doing; nothing here counts what is left.
export function tableHeaderStyleCommand(id: string, field: TableHeaderStyleField, operation: 'set' | 'clear', value = ''): ArrayBuffer {
  const encoded = ARRAY_HEADER_STYLE_FIELDS.includes(field) ? jsonArray(value === '' ? [] : value.split(',').map(jsonString))
    : NUMERIC_HEADER_STYLE_FIELDS.includes(field) ? jsonNumber(value)
    : jsonString(value)
  return commandBytes('updateTableHeaderStyle', [['id', jsonString(id)], ['field', jsonString(field)], ...operationFields(operation, encoded)])
}

// SPEC-table-rules' TWO KINDS, on the two shapes this module already has.
//
// `setTableMinHeight` is `setTableHeaderHeight`'s shape WITH a clear, and that
// difference is the format's: `headerHeight` is required and `minHeight` is
// optional, so "no floor" is a state an author must be able to get back to.
//
// `updateTableRules` is `updateTableHeaderStyle`'s shape — one attribute per
// command, never the block whole, for the same reason stated there: a block
// `set` would have to re-transmit the attributes the author did not touch,
// read back from the projection across an async boundary.
export type TableRulesField = 'width' | 'color' | 'between'

// `width` travels as a NUMBER (Go decodes it with a length decoder) and
// `between` as an ARRAY of boundary names (a plain json.Unmarshal into
// []string, the same decoder `border.edges` uses). The caller passes `between`
// comma-joined, which is the spelling the PROJECTION uses for the same set, so
// the panel never converts between two shapes of one value.
const NUMERIC_TABLE_RULES_FIELDS: ReadonlyArray<TableRulesField> = ['width']
const ARRAY_TABLE_RULES_FIELDS: ReadonlyArray<TableRulesField> = ['between']

export function tableMinHeightCommand(id: string, operation: 'set' | 'clear', value = ''): ArrayBuffer {
  return commandBytes('setTableMinHeight', [['id', jsonString(id)], ...operationFields(operation, jsonNumber(value))])
}

export function tableRulesCommand(id: string, field: TableRulesField, operation: 'set' | 'clear', value = ''): ArrayBuffer {
  const encoded = ARRAY_TABLE_RULES_FIELDS.includes(field) ? jsonArray(value === '' ? [] : value.split(',').map(jsonString))
    : NUMERIC_TABLE_RULES_FIELDS.includes(field) ? jsonNumber(value)
    : jsonString(value)
  return commandBytes('updateTableRules', [['id', jsonString(id)], ['field', jsonString(field)], ...operationFields(operation, encoded)])
}

// The `{op[, value]}` tail both clearable kinds share. A clear carries NO
// value, and that is arity rather than politeness: Go counts every top-level
// key and refuses any other count, so a clear that carried one would be refused
// whole.
function operationFields(operation: 'set' | 'clear', encoded: string): ReadonlyArray<JsonField> {
  return operation === 'clear' ? [['op', jsonString('clear')]] : [['op', jsonString('set')], ['value', encoded]]
}
