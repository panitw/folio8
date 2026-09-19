import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { updateComponentPropertiesCommand, type PropertyIntent } from './component-property-command'

const decode = (value: ArrayBuffer): string => new TextDecoder().decode(value)

describe('updateComponentPropertiesCommand', () => {
  it('encodes one opaque versioned command without a document model', () => {
    expect(decode(updateComponentPropertiesCommand(['e1', 'e2'], { field: 'x', operation: 'set', value: '12.125' }))).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1","e2"],"changes":{"x":{"op":"set","value":12.125}}}')
  })

  it('keeps clear distinct from a literal empty text value', () => {
    expect(decode(updateComponentPropertiesCommand(['e1'], { field: 'visibleIf', operation: 'clear' }))).toContain('"op":"clear"')
    expect(decode(updateComponentPropertiesCommand(['e1'], { field: 'value', operation: 'set', value: '' }))).toContain('"value":""')
  })

  // Story 7.4. Two encodings that are easy to get wrong in opposite ways.
  it('sends a ratio unquoted and a multi-line clause with its breaks escaped', () => {
    // lineSpacing is a RAW, UNQUOTED number carrying the author's own ratio.
    // Go multiplies by 1000 itself, so 1.5 becomes 1500 thousandths in the
    // document; sending 1500 would be refused as 1 500 000, outside the
    // load-time range, and sending "1.5" quoted is refused as a non-number.
    expect(decode(updateComponentPropertiesCommand(['e1'], { field: 'lineSpacing', operation: 'set', value: '1.5' }))).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"lineSpacing":{"op":"set","value":1.5}}}')
    expect(decode(updateComponentPropertiesCommand(['e1'], { field: 'lineSpacing', operation: 'clear' }))).toContain('"lineSpacing":{"op":"clear"}')
    // A clause's paragraph breaks are QUOTED text and survive as \n escapes;
    // a CRLF pair travels as it was typed, and the engine folds it into ONE
    // mandatory break rather than two.
    expect(decode(updateComponentPropertiesCommand(['e1'], { field: 'value', operation: 'set', value: 'One.\nTwo.\r\nThree.' }))).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"value":{"op":"set","value":"One.\\nTwo.\\r\\nThree."}}}')
  })

  // Story 8.2. quote() escaped `\ " \n \r \t` and NOTHING else, while JSON
  // requires every code point in U+0000-U+001F. A value carrying any other C0
  // control — U+0001 from the prose field's paste path, most plausibly — put a
  // RAW control byte inside a JSON string, so the command was malformed before
  // Go could read the field and the engine answered with a generic parse
  // failure instead of the located refusal naming it. Routing quote() through
  // JSON.stringify WIDENS what is escaped and must not narrow it, so the five
  // it already handled are re-asserted here beside the ones it missed.
  it('escapes the whole of U+0000-U+001F, and still escapes the five it always did', () => {
    const payload = (value: string) => decode(updateComponentPropertiesCommand(['e1'], { field: 'value', operation: 'set', value }))
    expect(() => JSON.parse(payload('a\u0001b'))).not.toThrow()
    expect(JSON.parse(payload('a\u0001b')).changes.value.value).toBe('a\u0001b')
    expect(payload('a\u0001b')).toContain('\\u0001')
    expect(payload('\u0000\u001f')).toContain('\\u0000\\u001f')
    // The departed population: the five characters quote() already handled,
    // and a value made only of JSON's own syntax characters.
    expect(payload('a\\b"c\nd\re\tf')).toContain('"value":"a\\\\b\\"c\\nd\\re\\tf"')
    // A lone surrogate is well-formed JSON only when escaped; it used to
    // travel raw and be replaced by U+FFFD at the encoder.
    expect(() => JSON.parse(payload('a\uD800b'))).not.toThrow()
    // A field name and an operation take the same encoder, unchanged.
    expect(payload('x')).toContain('"changes":{"value":{"op":"set"')
  })

  // STORY 14.2. THE WIDENING IS ADDITIVE, AND THIS IS WHERE THAT IS PROVED
  // RATHER THAN CLAIMED.
  //
  // The singular form's bytes are re-asserted VERBATIM beside the new form's,
  // because "additive" is a statement about the old call shape and nothing
  // else can check it: thirteen construction sites still pass one intent, and
  // if any of their bytes had moved, every gesture in the panel would have
  // changed meaning while the type checker stayed silent.
  it('encodes a single intent exactly as it did before the array form existed', () => {
    expect(decode(updateComponentPropertiesCommand(['e1'], { field: 'height', operation: 'set', value: '2' }))).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"height":{"op":"set","value":2}}}')
    expect(decode(updateComponentPropertiesCommand(['e1'], { field: 'background', operation: 'set', value: '#0b1120' }))).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"background":{"op":"set","value":"#0b1120"}}}')
    // A ONE-ELEMENT ARRAY IS THE SAME BYTES AS THE BARE INTENT. That is what
    // makes the two forms one encoder rather than two, and it is why no call
    // site had to be converted to reach the new capability.
    expect(decode(updateComponentPropertiesCommand(['e1'], [{ field: 'height', operation: 'set', value: '2' }]))).toBe(decode(updateComponentPropertiesCommand(['e1'], { field: 'height', operation: 'set', value: '2' })))
  })

  it('carries several changes in one object, one key per intent, each encoded by its own field rule', () => {
    expect(decode(updateComponentPropertiesCommand(['e1'], [{ field: 'width', operation: 'set', value: '1' }, { field: 'height', operation: 'set', value: '72' }]))).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"width":{"op":"set","value":1},"height":{"op":"set","value":72}}}')
    // Each intent keeps its OWN encoding rule — the point fields unquoted, a
    // colour quoted, a clear carrying no value at all — so a mixed batch is
    // not a second, looser encoder wearing the same name.
    expect(decode(updateComponentPropertiesCommand(['e1'], [{ field: 'width', operation: 'set', value: '1' }, { field: 'background', operation: 'set', value: '#000000' }, { field: 'visibleIf', operation: 'clear' }]))).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"width":{"op":"set","value":1},"background":{"op":"set","value":"#000000"},"visibleIf":{"op":"clear"}}}')
    // The authority already refuses a duplicate key at any nesting level, so
    // the widened form cannot compose a command whose meaning is decided by
    // last-wins. Asserted, not assumed: this is the one new way to reach it.
    expect(() => updateComponentPropertiesCommand(['e1'], [{ field: 'width', operation: 'set', value: '1' }, { field: 'width', operation: 'set', value: '2' }])).toThrow(/twice in one object/)
  })

  // GUARDRAIL 3 — THE ORDERING PROOF, AND IT IS A MIRROR RATHER THAN A
  // RESTATEMENT.
  //
  // `changes` is a JSON OBJECT. If the engine walked it in insertion order,
  // `{width, height}` and `{height, width}` would be two different documents
  // and this encoder would have quietly acquired an ordering dependency the
  // caller has no way to know about. It does not: Go decodes `changes` into a
  // `map[string]json.RawMessage` — which has no insertion order to walk — and
  // then iterates its own `propertyOrder`. Both halves are read out of the Go
  // source here, because the whole hazard is that the TypeScript side cannot
  // see the Go side and a comment asserting it would go on asserting it after
  // the engine moved.
  it('leaves wire key order meaningless, and reads the engine\'s own source to say so', () => {
    const forward = decode(updateComponentPropertiesCommand(['e1'], [{ field: 'width', operation: 'set', value: '1' }, { field: 'height', operation: 'set', value: '72' }]))
    const reversed = decode(updateComponentPropertiesCommand(['e1'], [{ field: 'height', operation: 'set', value: '72' }, { field: 'width', operation: 'set', value: '1' }]))
    // The bytes DO differ — the encoder writes what the caller listed — and
    // the two commands mean the same thing.
    expect(forward).not.toBe(reversed)
    expect(JSON.parse(forward)).toEqual(JSON.parse(reversed))
    const go = fs.readFileSync(path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../folio-go/component_commands.go'), 'utf8')
    // The decode target is a MAP, so JSON member order is discarded before any
    // change is applied.
    expect(go).toContain('var changes map[string]json.RawMessage')
    // And the apply loop walks a declared order of its own.
    expect(go).toContain('propertyOrder := []string{"x", "y", "width", "height",')
    expect(go).toContain('for _, key := range propertyOrder {')
    // Non-vacuity in the other direction: the strings above are real matches,
    // not a pair of assertions that would pass against any file.
    expect(go).not.toContain('var changes []json.RawMessage')
  })

  // PATCH 7 — THE EMPTY BATCH IS UNREPRESENTABLE, AND THIS IS THE PROOF.
  //
  // `updateComponentPropertiesCommand(ids, [])` would emit `"changes":{}`. Go
  // refuses it (`len(changes) == 0`) — and the panel would SWALLOW that refusal
  // in silence, because an error is anchored by the fields the intent carried
  // and an empty intent carries none, so `errorFor` matches no control and the
  // author sees nothing at all. A silently swallowed refusal is worse than a
  // crash.
  //
  // ⚠ THIS ASSERTION IS THE `@ts-expect-error` ITSELF, not the expect() below.
  // TypeScript reds when a line marked `@ts-expect-error` compiles CLEANLY, so
  // the day `PropertyIntents` stops being a non-empty tuple, `npx tsc -b` fails
  // here. A runtime throw would be the weaker guard: it can be reached, and a
  // guard that can be reached is a guard that has to be tested for. The call is
  // never invoked — the type is the whole of the check.
  it('cannot be handed an empty batch: the type refuses it before the encoder is reached', () => {
    const empty: ReadonlyArray<PropertyIntent> = []
    // @ts-expect-error an empty array is not `readonly [PropertyIntent, ...PropertyIntent[]]`
    const refused = () => updateComponentPropertiesCommand(['e1'], empty)
    expect(typeof refused).toBe('function')
    // And the one-element tuple, which is the smallest LEGAL batch, compiles and
    // encodes — so the type is narrow rather than merely hostile.
    expect(decode(updateComponentPropertiesCommand(['e1'], [{ field: 'width', operation: 'clear' }]))).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"width":{"op":"clear"}}}')
  })

  it('keeps the explicit format null operation distinct from clear and set', () => {
    expect(decode(updateComponentPropertiesCommand(['e1'], { field: 'background', operation: 'null' }))).toContain('"background":{"op":"null"}')
    expect(decode(updateComponentPropertiesCommand(['e1'], { field: 'borderEdges', operation: 'set', value: ['top', 'bottom'] }))).toContain('"value":["top","bottom"]')
  })
})
