import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { commandBytes, jsonArray, jsonBoolean, jsonNumber, jsonObject, jsonString } from './command-json'
import { bindComponentScalarCommand, deleteComponentCommand, moveComponentCommand } from './component-command'
import { setComponentAssetCommand } from './component-asset-command'
import { updateComponentPropertiesCommand } from './component-property-command'
import { acceptSampleData } from './sample-data'

const text = (value: ArrayBuffer): string => new TextDecoder().decode(value)
const parsed = (value: ArrayBuffer): Record<string, unknown> => JSON.parse(text(value)) as Record<string, unknown>

// U+1F600 is one code point and TWO UTF-16 units. Every escaping test that
// preceded this one used BMP-only inputs, so the defect below was invisible to
// all of them.
const EMOJI = '\u{1F600}'

describe('the command-JSON authority', () => {
  it('emits numbers unquoted and strings quoted, with the envelope Go counts', () => {
    expect(text(commandBytes('demoCommand', [['id', jsonString('e1')], ['width', jsonNumber('13')], ['snap', jsonBoolean(true)]])))
      .toBe('{"kind":"demoCommand","version":1,"id":"e1","width":13,"snap":true}')
    // The three literal number forms the shipped assertions pin, unchanged.
    expect(jsonNumber('13')).toBe('13')
    expect(jsonNumber('1.5')).toBe('1.5')
    expect(jsonNumber('0.001')).toBe('0.001')
    expect(jsonNumber(-12.125)).toBe('-12.125')
  })

  it('yields null for a draft that is not a number, and for a box the author emptied', () => {
    // Go holds the numeric grammar and answers with its own located sentence.
    // The encoder's job is to make bytes that REACH that rule.
    expect(jsonNumber('abc')).toBe('null')
    expect(jsonNumber('')).toBe('null')
    expect(jsonNumber('   ')).toBe('null')
    expect(jsonNumber(Number.NaN)).toBe('null')
    // Number('') is 0 in JavaScript. Sending 0 for an emptied box would be the
    // encoder inventing a value, which is the class of defect it exists to stop.
    expect(jsonNumber('')).not.toBe('0')
  })

  // THE COERCION TABLE, PINNED. This is the regression guard for the defect this
  // encoder was rewritten to remove: routing the numeric path through
  // `Number()` silently WIDENED the accept-set of both exported doors, in a
  // story whose whole purpose is to narrow them. Every left-hand value below
  // reached the wire as the middle value under that encoder, and every one of
  // them had earned a located refusal from Go before.
  it('never re-computes a numeric draft, so the engine sees what the author typed', () => {
    for (const [draft, coerced] of [
      ['1e3', '1000'], ['0x10', '16'], ['0b101', '5'], ['0o17', '15'],
      ['.5', '0.5'], ['5.', '5'], ['007', '7'], ['+5', '5'],
      [' 12 ', '12'], ['\n7', '7'], ['1.0e2', '100'],
    ] as const) {
      expect(jsonNumber(draft)).not.toBe(coerced)
      // Either it is a JSON number and travels VERBATIM, or it is not one and
      // becomes `null`. There is no third outcome and no rewriting.
      expect([draft, 'null']).toContain(jsonNumber(draft))
    }
    // The valid ones pass through byte-for-byte, the invalid ones do not travel.
    expect(jsonNumber('1e3')).toBe('1e3')
    expect(jsonNumber('0x10')).toBe('null')
    expect(jsonNumber('.5')).toBe('null')
    expect(jsonNumber('007')).toBe('null')
    expect(jsonNumber('+5')).toBe('null')
    expect(jsonNumber(' 12 ')).toBe('null')
    // Values both encoders already agreed to refuse. They are listed apart from
    // the table above because a row whose "coerced" and "correct" answers are
    // the same asserts nothing, and would have quietly padded the count.
    for (const refused of ['abc', '', '   ', '1_000', 'Infinity', 'NaN', '--1', '1.2.3', '0x', '1e', 'true']) expect(jsonNumber(refused)).toBe('null')
  })

  it('treats 1e3 and 1e21 the same as each other, which is the regression signature', () => {
    // ONE INPUT CLASS, ONE OUTCOME. Under the coercing encoder `1e3` became
    // `1000` and was ACCEPTED while `1e21` stayed `1e+21` and was refused —
    // the same class of literal splitting two ways, which is never a designed
    // behaviour and is the cheapest thing to notice.
    expect(jsonNumber('1e3')).toBe('1e3')
    expect(jsonNumber('1e21')).toBe('1e21')
    expect(jsonNumber('1E3')).toBe('1E3')
    expect(jsonNumber('1e-7')).toBe('1e-7')
    // Both reach Go, and Go alone decides — it refuses exponents with its own
    // located sentence. The encoder's job is to make bytes that REACH that
    // rule, not to duplicate or pre-empt it.
    for (const exponent of ['1e3', '1e21', '1E3', '1e-7']) expect(jsonNumber(exponent)).not.toBe('null')
  })

  it('carries an integer past 2^53 to the engine without losing a digit', () => {
    // A Number() round trip silently yields ...992 for this literal. Go reads
    // the literal, not a float64, so verbatim is the only correct answer.
    expect(jsonNumber('9007199254740993')).toBe('9007199254740993')
    expect(jsonNumber('9007199254740993')).not.toBe('9007199254740992')
    expect(jsonNumber('-9007199254740993')).toBe('-9007199254740993')
  })

  it('leaves a trailing zero exactly where the author put it', () => {
    // `0.100` is a valid JSON number and means what Go says it means. A
    // Number() round trip rewrote it to `0.1` — harmless here, and the same
    // mechanism that rewrote `1e3` to `1000`.
    expect(jsonNumber('0.100')).toBe('0.100')
    expect(jsonNumber('-0.0')).toBe('-0.0')
  })

  it('spells a caller-supplied JS number exactly as the shipped assertions pin it', () => {
    // The other input type on this path: geometry the panel computed, not a
    // draft the author typed. String() and JSON.stringify agree on every finite
    // value, and both refuse to spell the non-finite ones.
    for (const value of [13, 1.5, 0.001, -12.125, 0, 72, 24, 1e21, 1e-7]) expect(jsonNumber(value)).toBe(JSON.stringify(value))
    expect(jsonNumber(Number.NaN)).toBe('null')
    expect(jsonNumber(Number.POSITIVE_INFINITY)).toBe('null')
    expect(jsonNumber(Number.NEGATIVE_INFINITY)).toBe('null')
  })

  it('cannot build an object that declares the same key twice', () => {
    // The engine now REFUSES duplicate-key bytes at both exported doors, so an
    // authority that could still emit them would leave the designer able to
    // compose a command whose meaning is decided by last-wins.
    expect(() => jsonObject([['op', jsonString('set')], ['op', jsonString('clear')]])).toThrow(/twice/)
    expect(() => commandBytes('demoCommand', [['id', jsonString('e1')], ['id', jsonString('e2')]])).toThrow(/twice/)
    // A key colliding with the envelope's own is caught by the same rule.
    expect(() => commandBytes('demoCommand', [['version', jsonNumber(2)]])).toThrow(/twice/)
    // And the ordinary case is unaffected.
    expect(jsonObject([['a', jsonNumber(1)], ['b', jsonNumber(2)]])).toBe('{"a":1,"b":2}')
  })

  it('nests an object and an array without either level writing a brace', () => {
    expect(jsonObject([['op', jsonString('set')], ['value', jsonNumber('4')]])).toBe('{"op":"set","value":4}')
    expect(jsonArray([jsonString('a'), jsonString('b')])).toBe('["a","b"]')
    // A key is quoted by the same quoter as a value: a crafted field name
    // cannot open a second key either.
    expect(jsonObject([['a","b', jsonNumber(1)]])).toBe('{"a\\",\\"b":1}')
  })
})

describe('non-BMP text survives the wire (DW-75)', () => {
  it('round-trips U+1F600 through a bind segment', () => {
    const payload = bindComponentScalarCommand('e1', [`a${EMOJI}b`])
    const segments = (parsed(payload) as { segments: string[] }).segments
    expect(segments).toEqual([`a${EMOJI}b`])
    expect([...(segments[0] as string)]).toEqual(['a', EMOJI, 'b'])
    // The defect it replaced: `charCodeAt(0)` of a code point read only the
    // HIGH surrogate, escaped that alone, and never visited the low unit. The
    // result PARSED — so nothing anywhere reported an error — and the emoji
    // arrived as U+61 U+D83D U+62 for Go to replace with U+FFFD.
    expect(text(payload)).not.toContain('\\ud83db')
  })

  it('round-trips U+1F600 through an asset key and media type', () => {
    const payload = setComponentAssetCommand(`asset${EMOJI}`, `image/${EMOJI}`, new Uint8Array([1, 2, 3]).buffer)
    const command = parsed(payload) as { id: string; mediaType: string }
    expect(command.id).toBe(`asset${EMOJI}`)
    expect(command.mediaType).toBe(`image/${EMOJI}`)
    expect([...command.id]).toEqual(['a', 's', 's', 'e', 't', EMOJI])
  })

  it('round-trips U+1F600 through a property value', () => {
    const payload = updateComponentPropertiesCommand([`e${EMOJI}`], { field: 'value', operation: 'set', value: `hello ${EMOJI}` })
    const command = parsed(payload) as { ids: string[]; changes: { value: { value: string } } }
    expect(command.ids).toEqual([`e${EMOJI}`])
    expect(command.changes.value.value).toBe(`hello ${EMOJI}`)
  })
})

describe('a command names exactly what it says it names (DW-32)', () => {
  // The story's own payload, typed into a numeric field and blurred. It is NOT
  // `abc`: `abc` proves only that malformed bytes come out, while this one used
  // to produce VALID JSON whose parse collapsed to a different target and a
  // different change, with the author's own selection gone entirely.
  const INJECTION = '0}},"ids":["other"],"changes":{"width":{"op":"set","value":10'

  // THE TABLE IS DERIVED FROM THE ENCODER'S OWN SETS, not transcribed beside
  // them. A transcription is a second list to forget: the first draft of this
  // suite named seven fields while `pointFields` held ten, so three quarters of
  // the numeric surface went unexercised by a suite that read as exhaustive.
  //
  // It is read out of the source with anchored regexps rather than imported,
  // which is the mechanism `engine-bounds-mirror.test.ts` already uses on this
  // same file: the two Sets are module-private, and exporting them purely to be
  // testable would add a production export with no production reader.
  const numericFields = numericFieldsFromSource()

  // TITLED BY POSITION, NOT BY FIELD NAME, and deliberately.
  //
  // The derived set necessarily includes four fields this story's contract
  // forbids naming in any test name or assertion — Story 12.4 owns them and a
  // test spelling them here would turn that story's new guard into a change to
  // a passing acceptance. Deriving the table is required; naming its members is
  // forbidden; titling by position satisfies both, and the field a failing case
  // was about is still in the payload the runner prints.
  numericFields.forEach((field, index) => {
    it(`refuses to grow a second ids or changes key from a draft typed into numeric field ${index + 1} of ${numericFields.length}`, () => {
      const payload = text(updateComponentPropertiesCommand(['e1'], { field, operation: 'set', value: INJECTION }))
      const command = JSON.parse(payload) as { ids: string[]; changes: Record<string, unknown> }
      expect(command.ids).toEqual(['e1'])
      expect(Object.keys(command.changes)).toEqual([field])
      // Executed at the baseline: the payload parsed to ids:["other"] with a
      // width change the author never made.
      expect(payload).not.toContain('"other"')
      expect(payload.match(/"ids"/g)).toHaveLength(1)
      expect(payload.match(/"changes"/g)).toHaveLength(1)
    })
  })

  it('pins x and fontSize by name, against the derived table it also floors', () => {
    // NON-VACUITY FOR THE EXTRACTION, not a restatement of a literal declared
    // twenty lines up. `numericFields` is read out of another file, so these
    // two are the floor that catches a regexp which has quietly stopped
    // matching — the failure mode that would otherwise make every derived case
    // above pass by iterating an empty list. Both are named literals Story 12.4
    // never touches.
    expect(numericFields.length).toBeGreaterThanOrEqual(10)
    expect(numericFields).toContain('x')
    expect(numericFields).toContain('fontSize')
    expect(numericFields).toContain('lineSpacing')
    for (const field of ['x', 'fontSize'] as const) {
      const payload = text(updateComponentPropertiesCommand(['e1'], { field, operation: 'set', value: INJECTION }))
      expect(payload).toBe(`{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"${field}":{"op":"set","value":null}}}`)
    }
  })

  // THE DOCUMENT-CONTENT LEG, PROVED END TO END RATHER THAN ASSERTED.
  //
  // A bind segment is a JSON object KEY taken verbatim out of the author's own
  // sample-data file (sample-data.ts's DiscoveryParser), and nothing constrains
  // what characters a JSON key may hold. So this route needs NO TYPING: open a
  // data file, click the node, press Connect — and before this story the key
  // arrived at the engine as a lone surrogate, silently binding to a path the
  // author never picked. The command is an ADDRESS, not display text, which is
  // why the corruption is a wrong target rather than mojibake.
  it('carries a non-BMP key out of a sample-data file and onto the wire intact', () => {
    const document = `{"customer":{"na${EMOJI}me":"Ada"}}`
    const sample = acceptSampleData('data.json', new TextEncoder().encode(document).buffer)
    const leaf = sample.tree.children[0]?.children[0]
    expect(leaf?.segments).toEqual(['customer', `na${EMOJI}me`])
    const payload = bindComponentScalarCommand('e1', leaf?.segments ?? [])
    expect((parsed(payload) as { segments: string[] }).segments).toEqual(['customer', `na${EMOJI}me`])
  })

  it('sends one id, the literal it was handed, for an id made of JSON syntax', () => {
    // HYGIENE, NOT THE SEVERITY LEG, and the distinction is stated here so a
    // later reader does not promote it. `component.id` was spliced into this
    // command RAW, as were `type` and `band` in its neighbours, and
    // quoting is the complete fix: the value becomes one string the engine
    // resolves literally, and a name it cannot find gets a located refusal.
    //
    // Such an id cannot arrive from an opened document —
    // internal/template/ids.go's validateElementID (AD-10/AC34) admits only
    // `^e[0-9a-z]+$` for every element id at parse time. The document-originated
    // leg is the bind segment above, and the whole gesture is proved in
    // App.test.tsx: accept a data file, click the node, press Connect.
    const hostile = 'a","id":"victim'
    const payload = text(deleteComponentCommand(hostile))
    const command = JSON.parse(payload) as { id: string }
    expect(command.id).toBe(hostile)
    expect(payload).toBe('{"kind":"deleteComponent","version":1,"id":"a\\",\\"id\\":\\"victim"}')
    // Executed at the baseline: `{"kind":"deleteComponent","version":1,"id":"a","id":"victim"}`
    // parsed, and last-wins at the engine resolved the id to `victim` — which
    // is also why the Go half refuses duplicate keys whatever produced them.
    expect(payload.match(/"id"/g)).toHaveLength(1)
    // The same id through a geometry command, which used to yield malformed
    // bytes rather than a wrong target.
    expect(JSON.parse(text(moveComponentCommand(hostile, 1000, 2000, false))).id).toBe(hostile)
  })
})

// numericFieldsFromSource reads `pointFields` and `ratioFields` out of
// component-property-command.ts — the two sets that decide, in production,
// which fields travel unquoted. Anchored on each declaration's own name,
// because `new Set<PropertyField>([...])` is the file's idiom and an unanchored
// match would read whichever set came first and compare the wrong list.
//
// A regexp that stops matching returns an empty list and would make every
// derived case vacuous, so the caller floors the count.
function numericFieldsFromSource(): ReadonlyArray<'x' | 'y' | 'width' | 'height' | 'fontSize' | 'borderWidth' | 'lineSpacing'> {
  const source = fs.readFileSync(path.join(path.dirname(fileURLToPath(import.meta.url)), 'component-property-command.ts'), 'utf8')
  const names = ['pointFields', 'ratioFields'].flatMap((declaration) => {
    const list = new RegExp(`^const ${declaration} = new Set<PropertyField>\\(\\[([^\\]]*)\\]\\)$`, 'm').exec(source)?.[1]
    if (list === undefined) throw new Error(`component-property-command.ts no longer declares ${declaration} where this test can read it; if it was restructured, re-derive this extraction rather than deleting the check`)
    return [...list.matchAll(/'([^']+)'/g)].map((match) => match[1] as string)
  })
  return names as ReadonlyArray<'x' | 'y' | 'width' | 'height' | 'fontSize' | 'borderWidth' | 'lineSpacing'>
}
