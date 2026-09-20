// This is an opaque Go command vocabulary, not a browser-side property or
// style model. A caller sends exact local literals only on an explicit commit.
//
// STORY 15.2a: every value now goes through command-json.ts, the one place
// that turns intent into JSON. `rawNumberLiteral` used to return the author's
// typed string VERBATIM and unquoted, so a draft carrying JSON syntax closed
// the command's own object and opened a second `ids` and a second `changes` —
// the attacker choosing both the target and the change, and the author's real
// selection disappearing. A numeric draft now travels as a JSON number
// BYTE-FOR-BYTE when it already is one, and as `null` when it is not — the
// author's literal is never re-computed. The engine holds the numeric grammar
// and answers with its own located sentence; the browser deliberately does not,
// and a `Number()` round trip here would be exactly that second authority,
// silently turning a typed `1e3` into a 1000pt width.
import type { JsonField } from './command-json'
import { commandBytes, commandFragment, jsonArray, jsonBoolean, jsonNumber, jsonObject, jsonString } from './command-json'

// STORY 14.2 TURNED THIS UNION INTO A LIST AND DERIVED THE UNION FROM IT.
//
// The panel now has to ask a RUNTIME question of it — `printsDataPath` in
// App.tsx asks whether the last segment of a diagnostic path is the name of a
// property field, because Go answers a multi-key refusal with the first key in
// canonical order rather than the key that failed, and `component.width` on a
// `{width, height}` intent is a mislabel while `component.geometry` is not.
// A type alone cannot answer that; a second, hand-written array beside the type
// could, and would drift from it silently.
//
// So there is ONE list, and the type is its members. Drift is unrepresentable
// rather than merely unlikely. The order is Go's `propertyOrder`
// (`component_commands.go`), member for member, which is where it always came
// from.
export const PROPERTY_FIELDS = ['x', 'y', 'width', 'height', 'value', 'expression', 'visibleIf', 'fontFamily', 'fontSize', 'lineSpacing', 'bold', 'italic', 'align', 'valign', 'color', 'background', 'borderWidth', 'borderColor', 'borderEdges', 'paddingTop', 'paddingRight', 'paddingBottom', 'paddingLeft', 'errorCorrection'] as const
export type PropertyField = typeof PROPERTY_FIELDS[number]
export const isPropertyField = (value: string): value is PropertyField => (PROPERTY_FIELDS as ReadonlyArray<string>).includes(value)

export type PropertyIntent = Readonly<{ field: PropertyField; operation: 'set' | 'clear' | 'null'; value?: string | boolean | ReadonlyArray<string> }>
// STORY 14.2 / PATCH 7 — THE EMPTY BATCH IS UNREPRESENTABLE, NOT MERELY
// UNTESTED.
//
// `updateComponentPropertiesCommand(ids, [])` would emit `"changes":{}`, which
// Go refuses (`len(changes) == 0`) — and the refusal would be SILENTLY
// SWALLOWED, because the panel anchors an error by the fields the intent
// carried and an empty intent carries none, so `errorFor` matches no control
// and the author is shown nothing at all. A non-empty tuple makes the call fail
// to compile instead. A guard asserted in a test is a guard that can be
// deleted; a guard in the type cannot be reached.
export type PropertyIntents = readonly [PropertyIntent, ...PropertyIntent[]]

const pointFields = new Set<PropertyField>(['x', 'y', 'width', 'height', 'fontSize', 'borderWidth', 'paddingTop', 'paddingRight', 'paddingBottom', 'paddingLeft'])
// lineSpacing travels unquoted like a point field but is NOT one, and the
// distinction is worth a second set rather than an entry in the first.
// It is a dimensionless RATIO. Go's decoder (template.DecodeLineSpacingRaw)
// reads the author's own literal and performs the x1000 to thousandths
// itself, exactly as it does for a `lineSpacing` written in a .folio file —
// so `1.5` on the wire is 1500 thousandths in the document, and sending
// `1500` would be refused as 1 500 000, outside the load-time range. Verified
// against the engine, not inferred: the two entry points share one decoder
// precisely so the inspector cannot mean something different from the file.
const ratioFields = new Set<PropertyField>(['lineSpacing'])

// STORY 17.4. The engine refuses a NON-POSITIVE length on exactly these four
// keys — `component_commands.go` spells the rule `(key == "width" || key ==
// "height" || key == "fontSize" || key == "borderWidth") && length <= 0`. `x`
// and `y` are absent from it and are unbounded, negatives included.
//
// The inspector's arrow step reads this list to know where to STOP, so a
// keypress can never propose a value the command path will refuse. That makes
// it an invariant duplicated across the Go/TypeScript boundary, which D-7.4.5
// says moves in one commit with a test that reads both sides:
// `engine-bounds-mirror.test.ts` reads this declaration and Go's predicate and
// asserts they name the same four keys.
export const POSITIVE_LENGTH_FIELDS: ReadonlyArray<PropertyField> = ['width', 'height', 'fontSize', 'borderWidth']

// STORY 17.4, ADDED AT REVIEW. THE `> 0` RULE ABOVE IS NOT THE ONLY BOUND ON
// THE PROPERTY PATH, and the story's Code Map cited only that one.
// `updateComponentPropertiesInPlace` calls `containComponent` on every id after
// applying the changes (`component_commands.go:880` -> `:1912`), and that
// predicate opens `x < 0 || y < 0 || width < 0 || height < 0`. So `x` and `y`
// are NOT unbounded below, as an earlier reading of this file's neighbour
// concluded — their floor is the band origin, zero. `width` and `height` are
// bounded more tightly by the `> 0` rule above, so this list names only the two
// fields whose floor comes from containment alone.
//
// `containComponent` ALSO bounds these fields ABOVE, against the band extents
// (`x > band.Width`, `width > band.Width - x`, and in the vertically capping
// bands `y > band.Height`, `height > band.Height - y`). Those are NOT mirrored
// here and the arrow step does not clamp to them — see the story's Spec Change
// Log, which records that as an open question rather than a settled omission.
export const ORIGIN_FLOOR_FIELDS: ReadonlyArray<PropertyField> = ['x', 'y']

// STORY 14.2 WIDENS THIS ADDITIVELY: one command may now carry SEVERAL changes.
//
// WHY, and it is one gesture rather than a general capability: a Line's
// orientation control swaps `width` and `height`, and those two numbers must
// move as ONE undo entry. Two sequential commands are two revisions, two undo
// entries, and — worse — a transient shape between them that
// `containComponent` gets to refuse, because the engine contains each id ONCE
// AFTER ALL CHANGES ARE APPLIED (`component_commands.go`: applyPropertyChanges
// then containComponent, per id). One command has no transient shape to refuse.
//
// NOTHING NEW REACHES GO. Its only cardinality rule on `changes` is a FLOOR —
// `len(changes) == 0` is refused, `> 1` is not — and every key this form can
// emit is already in `propertyOrder`. The engine was written for a multi-key
// `changes` object; this is the first caller to send one.
//
// THE SINGULAR FORM IS UNTOUCHED, BY CONSTRUCTION. `PropertyIntent` did not
// move, the parameter merely ACCEPTS an array as well, and a single intent
// still encodes byte-for-byte the bytes it encoded before — asserted in
// `component-property-command.test.ts`, not assumed. Thirteen call sites pass
// the singular form and none of them changed.
//
// KEY ORDER ON THE WIRE IS THE CALLER'S AND CARRIES NO MEANING. Go decodes
// `changes` into a `map[string]json.RawMessage` and walks its own
// `propertyOrder`, so `{width, height}` and `{height, width}` reach the same
// document. That is asserted against Go's own source in the test file rather
// than believed here. `jsonObject` throws on a duplicate key, so the same field
// cannot be named twice in one command whatever the caller intends.
// ONE FIELD LIST, TWO TERMINALS (spec-install-all-face-cuts story 2). The
// property commit now also travels INSIDE a unit — pressing B on a family whose
// bold is held sends the cut embed and this command as one undoable unit — and
// a unit's members are command objects nested in another command, so this
// builder needs a fragment twin beside its `ArrayBuffer` one.
//
// THE LIST IS NOT WRITTEN TWICE, AND THAT IS THE POINT RATHER THAN THE SAVING.
// The engine counts every top-level key and refuses any other arity, so two
// hand-copied field lists can drift by one key into a refusal the author meets
// only on whichever path was not under test. A command sent alone and the same
// command sent in a unit are the SAME BYTES, by construction.
const updateComponentPropertiesFields = (ids: ReadonlyArray<string>, intent: PropertyIntent | PropertyIntents): ReadonlyArray<JsonField> => {
  const intents = 'field' in intent ? [intent] : intent
  return [['ids', jsonArray(ids.map(jsonString))], ['changes', jsonObject(intents.map((one) => [one.field, changeFor(one)] as const))]]
}

export function updateComponentPropertiesCommand(ids: ReadonlyArray<string>, intent: PropertyIntent | PropertyIntents): ArrayBuffer {
  return commandBytes('updateComponentProperties', updateComponentPropertiesFields(ids, intent))
}

/** The same command as a fragment, for nesting inside a unit. See `updateComponentPropertiesFields`. */
export function updateComponentPropertiesFragment(ids: ReadonlyArray<string>, intent: PropertyIntent | PropertyIntents): string {
  return commandFragment('updateComponentProperties', updateComponentPropertiesFields(ids, intent))
}

function changeFor(intent: PropertyIntent): string {
  return intent.operation === 'clear' || intent.operation === 'null'
    ? jsonObject([['op', jsonString(intent.operation)]])
    : pointFields.has(intent.field) || ratioFields.has(intent.field)
      ? jsonObject([['op', jsonString('set')], ['value', numberLiteral(intent.value)]])
      : jsonObject([['op', jsonString('set')], ['value', propertyValue(intent.value)]])
}

function numberLiteral(value: PropertyIntent['value']): string {
  // Unquoted, and now encoded rather than spliced; Go alone decides whether it
  // is a valid number, in whatever unit that field is carried in. A value that
  // is not a typed literal at all cannot become a number, so it travels as the
  // same `null` an unparseable draft does rather than as malformed bytes.
  return jsonNumber(typeof value === 'string' ? value : Number.NaN)
}

function propertyValue(value: PropertyIntent['value']): string {
  if (typeof value === 'string') return jsonString(value)
  // Through the authority, not String(). A boolean is the one scalar whose JS
  // spelling happens to match its JSON spelling, which is exactly why an
  // exception for it would survive unnoticed until the day it did not.
  if (typeof value === 'boolean') return jsonBoolean(value)
  return jsonArray((value ?? []).map(jsonString))
}

// STORY 8.2 REPLACED A HAND-ROLLED ESCAPE TABLE HERE, and Story 15.2a moved
// what replaced it into command-json.ts. The record is worth keeping because
// it is the same defect class twice: the hand-rolled table escaped `\ " \n \r
// \t` and nothing else, while JSON requires every code point in U+0000-U+001F
// to be escaped. A value carrying any other C0 control — U+0001 from a paste,
// most plausibly — emitted a raw control byte inside a JSON string, so the
// command was MALFORMED BEFORE Go could read the field, and the engine answered
// with a generic parse failure instead of the located refusal naming the field.
// Engine-side validation cannot substitute for this: the bytes never reach the
// rule. JSON.stringify IS the escape table; it is never re-derived.
