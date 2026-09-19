import { describe, expect, it } from 'vitest'
import { tableAltRowBackgroundCommand, tableHeaderHeightCommand, tableHeaderStyleCommand, tableMinHeightCommand, tableRulesCommand } from './table-style-command'

// THIS FILE IS THE SINGLE AUTHORITY ON THESE WIRE BYTES — Story 12.3's seven
// header-style fields and Story 14.8's border trio alike — and the story
// spec deliberately does not restate the arity: the spec owns the op grammar
// and the shared `findComponent` gate, and pinning the arity in prose as well
// would be a second spelling of one rule — the defect Story 12.2 spent its
// whole life on. If the two ever disagreed, this file would be right.
//
// Order is part of the contract twice over: Go counts every top-level key
// (componentFields) and refuses any other count, and the factory's whole job is
// to be unable to build anything else.
const text = (value: ArrayBuffer): string => new TextDecoder().decode(value)
const keys = (value: ArrayBuffer): string[] => Object.keys(JSON.parse(text(value)) as Record<string, unknown>)

describe('tableHeaderHeightCommand', () => {
  it('encodes one opaque versioned command with the four top-level keys Go counts, in order', () => {
    expect(text(tableHeaderHeightCommand('e7', '18')))
      .toBe('{"kind":"setTableHeaderHeight","version":1,"id":"e7","height":18}')
    expect(text(tableHeaderHeightCommand('e7', '12.5')))
      .toBe('{"kind":"setTableHeaderHeight","version":1,"id":"e7","height":12.5}')
    expect(keys(tableHeaderHeightCommand('e7', '18'))).toEqual(['kind', 'version', 'id', 'height'])
  })

  it('has no clear to offer, and sends the author\'s own literal rather than a re-computation of it', () => {
    // There is no `op` on this kind at all — `headerHeight` is required by the
    // format, so a cleared one is a document that cannot be reopened. The
    // factory cannot build the command that would ask for it.
    expect(text(tableHeaderHeightCommand('e7', '18'))).not.toContain('"op"')
    expect(text(tableHeaderHeightCommand('e7', '1e3'))).toContain('"height":1e3')
    expect(text(tableHeaderHeightCommand('e7', '18.0001'))).toContain('"height":18.0001')
    // Number('') is 0 in JavaScript. An emptied box must not silently restore a
    // height nobody typed.
    expect(text(tableHeaderHeightCommand('e7', ''))).toBe('{"kind":"setTableHeaderHeight","version":1,"id":"e7","height":null}')
  })
})

describe('tableAltRowBackgroundCommand', () => {
  it('encodes set with a value and clear without one, in order', () => {
    expect(text(tableAltRowBackgroundCommand('e7', 'set', '#DDEEFF')))
      .toBe('{"kind":"setTableAltRowBackground","version":1,"id":"e7","op":"set","value":"#DDEEFF"}')
    expect(text(tableAltRowBackgroundCommand('e7', 'clear')))
      .toBe('{"kind":"setTableAltRowBackground","version":1,"id":"e7","op":"clear"}')
    expect(keys(tableAltRowBackgroundCommand('e7', 'set', '#DDEEFF'))).toEqual(['kind', 'version', 'id', 'op', 'value'])
    expect(keys(tableAltRowBackgroundCommand('e7', 'clear'))).toEqual(['kind', 'version', 'id', 'op'])
  })

  it('never spells `null`, and passes a malformed colour through for the engine to refuse', () => {
    // Clearing is the ZERO presence, which removes the key. `"altRowBackground":
    // null` would still be the key in the file — different bytes, an undo
    // entry, and a raised format version — and the serializer has no null
    // branch for it, so the loader would refuse the document it just wrote.
    expect(text(tableAltRowBackgroundCommand('e7', 'clear'))).not.toContain('null')
    // The panel invents no second validation: Go's parseHexColor is the gate
    // and its located sentence is what the author sees.
    expect(text(tableAltRowBackgroundCommand('e7', 'set', 'not-a-colour')))
      .toBe('{"kind":"setTableAltRowBackground","version":1,"id":"e7","op":"set","value":"not-a-colour"}')
  })
})

describe('tableHeaderStyleCommand', () => {
  it('encodes set and clear for one named field, in order', () => {
    expect(text(tableHeaderStyleCommand('e7', 'align', 'set', 'center')))
      .toBe('{"kind":"updateTableHeaderStyle","version":1,"id":"e7","field":"align","op":"set","value":"center"}')
    expect(text(tableHeaderStyleCommand('e7', 'align', 'clear')))
      .toBe('{"kind":"updateTableHeaderStyle","version":1,"id":"e7","field":"align","op":"clear"}')
    expect(keys(tableHeaderStyleCommand('e7', 'align', 'set', 'center'))).toEqual(['kind', 'version', 'id', 'field', 'op', 'value'])
    expect(keys(tableHeaderStyleCommand('e7', 'align', 'clear'))).toEqual(['kind', 'version', 'id', 'field', 'op'])
  })

  // ⚠ THE ENUMERATION IS THE POINT OF THIS TEST, so it must enumerate what the
  // MODULE encodes rather than what it once encoded. Story 14.8 took the union
  // from seven fields to TEN, and it did not add them evenly: the three JSON
  // shapes are now THREE numeric fields, SIX string fields and ONE array field.
  // A file that calls itself the single authority on these wire bytes and skips
  // three of the ten is not one.
  it('sends the three numeric fields unquoted, the six string fields quoted, and the one array field as an array', () => {
    // TRANSPORT, not validation. Go decodes fontSize with a length decoder,
    // align with a string one and border.edges with `json.Unmarshal` into
    // `[]string`; a value in the wrong JSON type could not reach any of those
    // rules to be judged by it.
    for (const [field, draft] of [['fontSize', '14'], ['lineSpacing', '1.5'], ['border.width', '0.5']] as const) {
      expect(text(tableHeaderStyleCommand('e7', field, 'set', draft)), field).toContain(`"value":${draft}`)
    }
    for (const field of ['fontFamily', 'background', 'color', 'valign', 'align', 'border.color'] as const) {
      expect(text(tableHeaderStyleCommand('e7', field, 'set', 'x')), field).toContain('"value":"x"')
    }
    // THE ONE ARRAY FIELD. The caller passes the edge names comma-joined — the
    // spelling the PROJECTION uses for the same set — and the module splits them,
    // so the panel never converts between two shapes of one value.
    expect(text(tableHeaderStyleCommand('e7', 'border.edges', 'set', 'top,bottom')))
      .toBe('{"kind":"updateTableHeaderStyle","version":1,"id":"e7","field":"border.edges","op":"set","value":["top","bottom"]}')
    // And an emptied numeric draft is `null`, never a 0 nobody typed.
    expect(text(tableHeaderStyleCommand('e7', 'fontSize', 'set', ''))).toContain('"value":null')
    // ⚠ BUT AN EMPTIED BORDER WIDTH IS `null` FOR A DIFFERENT REASON AND WITH A
    // DIFFERENT CONSEQUENCE, and it is worth pinning because ZERO IS LEGAL for
    // this one field: `0` is the thinnest device line PDF can draw, so a factory
    // that turned an empty draft into `0` would author a real border out of an
    // emptied box. `null` is refused whole by `tableCommandOp`, which is the
    // correct outcome — the panel sends `op: "clear"` to remove it.
    expect(text(tableHeaderStyleCommand('e7', 'border.width', 'set', ''))).toContain('"value":null')
    expect(text(tableHeaderStyleCommand('e7', 'border.width', 'set', '0'))).toContain('"value":0')
    // AND AN EMPTIED EDGE DRAFT IS `[]`, NOT A GUESS. Go refuses the empty array
    // with a located sentence; inventing `[""]` or dropping the command would be
    // this module holding a rule of its own, which it does not.
    expect(text(tableHeaderStyleCommand('e7', 'border.edges', 'set', ''))).toContain('"value":[]')
  })

  it('cannot be made to carry a second field or a second value from one typed string', () => {
    // The splice payload, typed into a colour box. A raw template literal would
    // have produced valid JSON here in which the field the command NAMES is not
    // the field it CHANGES.
    const payload = text(tableHeaderStyleCommand('e7', 'color', 'set', '#000000","field":"background'))
    const command = JSON.parse(payload) as Record<string, unknown>
    expect(command.field).toBe('color')
    expect(payload.match(/"field"/g)).toHaveLength(1)
    expect(Object.keys(command)).toEqual(['kind', 'version', 'id', 'field', 'op', 'value'])
    // And the same through the id, which is document-supplied rather than typed.
    const spliced = text(tableAltRowBackgroundCommand('e7","op":"clear', 'set', '#DDEEFF'))
    expect((JSON.parse(spliced) as Record<string, unknown>).op).toBe('set')
    expect(spliced.match(/"op"/g)).toHaveLength(1)
  })

  it('round-trips an astral character in a value without producing a lone surrogate', () => {
    const EMOJI = '\u{1F600}'
    const wire = text(tableHeaderStyleCommand('e7', 'fontFamily', 'set', `n${EMOJI}me`))
    expect([...((JSON.parse(wire) as Record<string, string>).value ?? '')]).toEqual(['n', EMOJI, 'm', 'e'])
    expect(wire).not.toContain('\\ud83d')
    expect(wire).not.toContain('\\uD83D')
  })
})

// SPEC-table-rules' two kinds. Exact bytes, because a mock-call assertion in the
// panel's tests is satisfied equally by an encoder that drifted.
describe('tableMinHeightCommand', () => {
  it('encodes set with an unquoted length and clear with no value, in order', () => {
    expect(text(tableMinHeightCommand('e7', 'set', '600')))
      .toBe('{"kind":"setTableMinHeight","version":1,"id":"e7","op":"set","value":600}')
    expect(text(tableMinHeightCommand('e7', 'set', '12.5')))
      .toBe('{"kind":"setTableMinHeight","version":1,"id":"e7","op":"set","value":12.5}')
    expect(text(tableMinHeightCommand('e7', 'clear')))
      .toBe('{"kind":"setTableMinHeight","version":1,"id":"e7","op":"clear"}')
    expect(keys(tableMinHeightCommand('e7', 'set', '600'))).toEqual(['kind', 'version', 'id', 'op', 'value'])
    expect(keys(tableMinHeightCommand('e7', 'clear'))).toEqual(['kind', 'version', 'id', 'op'])
  })
})

describe('tableRulesCommand', () => {
  it('encodes width as a number, color as a string, between as an array, and clear with no value', () => {
    expect(text(tableRulesCommand('e7', 'width', 'set', '0.5')))
      .toBe('{"kind":"updateTableRules","version":1,"id":"e7","field":"width","op":"set","value":0.5}')
    expect(text(tableRulesCommand('e7', 'color', 'set', '#336699')))
      .toBe('{"kind":"updateTableRules","version":1,"id":"e7","field":"color","op":"set","value":"#336699"}')
    expect(text(tableRulesCommand('e7', 'between', 'set', 'columns,rows')))
      .toBe('{"kind":"updateTableRules","version":1,"id":"e7","field":"between","op":"set","value":["columns","rows"]}')
    expect(text(tableRulesCommand('e7', 'between', 'set', 'columns')))
      .toBe('{"kind":"updateTableRules","version":1,"id":"e7","field":"between","op":"set","value":["columns"]}')
    expect(text(tableRulesCommand('e7', 'between', 'clear')))
      .toBe('{"kind":"updateTableRules","version":1,"id":"e7","field":"between","op":"clear"}')
    expect(text(tableRulesCommand('e7', 'width', 'clear')))
      .toBe('{"kind":"updateTableRules","version":1,"id":"e7","field":"width","op":"clear"}')
    expect(keys(tableRulesCommand('e7', 'color', 'set', '#336699'))).toEqual(['kind', 'version', 'id', 'field', 'op', 'value'])
    expect(keys(tableRulesCommand('e7', 'color', 'clear'))).toEqual(['kind', 'version', 'id', 'field', 'op'])
  })
})
