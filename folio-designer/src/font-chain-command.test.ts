import { describe, expect, it } from 'vitest'
import { addFontChainCommand, addFontChainEntryCommand, deleteFontChainCommand, embedFontFamilyCommand, moveFontChainEntryCommand, removeFontChainEntryCommand, renameFontChainCommand } from './font-chain-command'

const text = (payload: ArrayBuffer): string => new TextDecoder().decode(payload)
const parsed = (payload: ArrayBuffer): Record<string, unknown> => JSON.parse(text(payload)) as Record<string, unknown>

// The two values the engine will never see if the encoder is wrong. `a"b\c`
// exercises the two characters JSON's own syntax is made of; U+0001 is a C0
// control that the encoder this story replaced did NOT escape — it emitted the
// raw byte inside a JSON string, which is malformed, so Go answered with a
// generic parse failure instead of the located refusal naming the field. An
// author pastes such a byte far more easily than they type one.
const awkward = 'a"b\\c'
const controlled = 'a\u0001b'

// The twelve fields `embedFontFamily` requires, minus the one under test. The
// bytes are one byte of nothing: this file asserts the WIRE, and Go is what
// decides whether a face is readable.
const pick = { chain: 'Inter', family: 'Inter', style: 'Regular', licence: 'OFL-1.1', licenceText: 'terms', copyright: 'c', source: 'https://example.invalid', mediaType: 'font/ttf', bytes: new Uint8Array([0]).buffer, tail: [] as ReadonlyArray<string> }

describe('the six font chain commands are exactly the payloads the engine reads', () => {
  it('encodes every kind with its exact field arity', () => {
    // componentFields(raw, N) counts EVERY top-level key, `kind` and `version`
    // included, and an extra or missing one is a refusal rather than an
    // ignored field. These are the six counts folio-go's handlers require.
    expect(text(addFontChainCommand('heading', ['Noto Sans', 'Noto Sans Thai']))).toBe('{"kind":"addFontChain","version":1,"name":"heading","entries":["Noto Sans","Noto Sans Thai"]}')
    expect(text(renameFontChainCommand('body', 'text'))).toBe('{"kind":"renameFontChain","version":1,"name":"body","to":"text"}')
    expect(text(deleteFontChainCommand('body'))).toBe('{"kind":"deleteFontChain","version":1,"name":"body"}')
    expect(text(addFontChainEntryCommand('body', 1, 'Noto Sans SC'))).toBe('{"kind":"addFontChainEntry","version":1,"name":"body","index":1,"face":"Noto Sans SC"}')
    expect(text(moveFontChainEntryCommand('body', 2, 0))).toBe('{"kind":"moveFontChainEntry","version":1,"name":"body","from":2,"to":0}')
    expect(text(removeFontChainEntryCommand('body', 1))).toBe('{"kind":"removeFontChainEntry","version":1,"name":"body","index":1}')

    // STORY 11.4 — ROUTE C: an entry may be an OBJECT naming a face and the cuts
    // it has. THE FIELD COUNTS DO NOT MOVE, which is the whole reason route C
    // was taken over widening an arity: `entries` is one field whatever shape
    // its members are, so `componentFields(raw, 4)` is untouched in Go.
    expect(text(addFontChainCommand('Roboto', [{ face: 'Roboto', bold: 'Roboto Bold', italic: 'Roboto Italic', boldItalic: 'Roboto Bold Italic' }]))).toBe('{"kind":"addFontChain","version":1,"name":"Roboto","entries":[{"face":"Roboto","bold":"Roboto Bold","italic":"Roboto Italic","boldItalic":"Roboto Bold Italic"}]}')
    // AN ABSENT CUT IS AN ABSENT KEY, never an empty string: Go refuses `""` as
    // a variant, so a family with only a bold must emit only `bold`.
    expect(text(addFontChainCommand('x', [{ face: 'Noto Sans Thai', bold: 'Noto Sans Thai Bold' }]))).toBe('{"kind":"addFontChain","version":1,"name":"x","entries":[{"face":"Noto Sans Thai","bold":"Noto Sans Thai Bold"}]}')
    // AND A FAMILY WITH NO CUT STAYS A BARE STRING — the shape a variant-free
    // entry canonicalises to, which is what keeps such a document at 1.0.
    expect(text(addFontChainCommand('x', ['Noto Sans SC']))).toBe('{"kind":"addFontChain","version":1,"name":"x","entries":["Noto Sans SC"]}')
    // THE TAIL TAKES THE SAME SHAPE, through the same encoder.
    expect(text(embedFontFamilyCommand({ ...pick, tail: [{ face: 'Noto Sans Thai', bold: 'Noto Sans Thai Bold' }, 'Noto Sans SC'] }))).toContain('"tail":[{"face":"Noto Sans Thai","bold":"Noto Sans Thai Bold"},"Noto Sans SC"]')

    const arity: ReadonlyArray<readonly [ArrayBuffer, number, readonly string[]]> = [
      [addFontChainCommand('heading', ['Noto Sans']), 4, ['kind', 'version', 'name', 'entries']],
      // THE SAME COUNT AND THE SAME KEYS with an object entry. A row that only
      // ever passed bare strings could not see route C move an arity.
      [addFontChainCommand('heading', [{ face: 'Roboto', bold: 'Roboto Bold' }]), 4, ['kind', 'version', 'name', 'entries']],
      [embedFontFamilyCommand({ ...pick, tail: [{ face: 'Noto Sans Thai', bold: 'Noto Sans Thai Bold' }] }), 12, ['kind', 'version', 'name', 'family', 'style', 'licence', 'licenceText', 'copyright', 'source', 'mediaType', 'data', 'tail']],
      [renameFontChainCommand('body', 'text'), 4, ['kind', 'version', 'name', 'to']],
      [deleteFontChainCommand('body'), 3, ['kind', 'version', 'name']],
      [addFontChainEntryCommand('body', 0, 'Noto Sans'), 5, ['kind', 'version', 'name', 'index', 'face']],
      [moveFontChainEntryCommand('body', 0, 1), 5, ['kind', 'version', 'name', 'from', 'to']],
      [removeFontChainEntryCommand('body', 0), 4, ['kind', 'version', 'name', 'index']],
    ]
    for (const [payload, fields, keys] of arity) {
      const object = parsed(payload)
      expect(Object.keys(object)).toEqual([...keys])
      expect(Object.keys(object)).toHaveLength(fields)
      expect(object.version).toBe(1)
    }
  })

  it('produces valid JSON for a value carrying a quote, a backslash and a C0 control, per builder', () => {
    // Every builder, every author-supplied position: the name, the rename
    // destination, a face in the create list, and a face added later.
    for (const value of [awkward, controlled, `${awkward}${controlled}`]) {
      expect(parsed(addFontChainCommand(value, [value]))).toEqual({ kind: 'addFontChain', version: 1, name: value, entries: [value] })
      expect(parsed(renameFontChainCommand(value, `${value}!`))).toEqual({ kind: 'renameFontChain', version: 1, name: value, to: `${value}!` })
      expect(parsed(deleteFontChainCommand(value))).toEqual({ kind: 'deleteFontChain', version: 1, name: value })
      expect(parsed(addFontChainEntryCommand(value, 3, value))).toEqual({ kind: 'addFontChainEntry', version: 1, name: value, index: 3, face: value })
      expect(parsed(moveFontChainEntryCommand(value, 1, 0))).toEqual({ kind: 'moveFontChainEntry', version: 1, name: value, from: 1, to: 0 })
      expect(parsed(removeFontChainEntryCommand(value, 2))).toEqual({ kind: 'removeFontChainEntry', version: 1, name: value, index: 2 })
    }
    // The C0 control travels as its \u escape, not as a raw byte.
    expect(text(deleteFontChainCommand(controlled))).toContain('\\u0001')
    expect(text(deleteFontChainCommand(controlled))).not.toContain('\u0001')
  })

  it('measures the departed encoder rather than asserting the new one is better', () => {
    // The escape table component-property-command.ts's quote() carried before
    // this story, reproduced here so the fix is proved against what it
    // replaced. It handles `\ " \n \r \t` and NOTHING else, while JSON
    // requires all of U+0000-U+001F.
    const departed = (value: string) => `"${value.replaceAll('\\', '\\\\').replaceAll('"', '\\"').replaceAll('\n', '\\n').replaceAll('\r', '\\r').replaceAll('\t', '\\t')}"`
    // It was right about the five it knew, which is the population that must
    // not be narrowed by the fix — JSON.stringify still escapes all of them.
    for (const value of [awkward, 'a\nb', 'a\rb', 'a\tb', 'a\\b', 'a"b']) {
      expect(departed(value)).toBe(JSON.stringify(value))
      expect(JSON.parse(departed(value))).toBe(value)
    }
    // And wrong about every other control character, which is the defect.
    expect(() => JSON.parse(`{"name":${departed(controlled)}}`)).toThrow()
    expect(JSON.parse(`{"name":${JSON.stringify(controlled)}}`)).toEqual({ name: controlled })
    // A lone surrogate is the second value the departed table passed through
    // raw. JSON.stringify escapes it into a well-formed document instead.
    expect(text(deleteFontChainCommand('a\uD800b'))).toContain('\\ud800')
    expect(() => JSON.parse(text(deleteFontChainCommand('a\uD800b')))).not.toThrow()
  })
})

// STORY 8.6 — THE PICK, ON THE WIRE.
//
// THE FIELD COUNT IS THE CONTRACT and it is asserted as a count, not just as a
// set of present keys: Go's componentFields(raw, 12) refuses a payload with an
// extra field outright, so a builder that grew a thirteenth would be refused
// in full rather than have the surplus ignored — and the refusal would arrive
// as an unlocated arity error that says nothing about which key was new.
describe('the embed command carries everything the document must record', () => {
  const face = {
    chain: 'Inter',
    family: 'Inter',
    style: 'Regular',
    licence: 'OFL-1.1',
    licenceText: 'Copyright (c) 2020 The Inter Project Authors\n\nThis Font Software is licensed under the SIL Open Font License, Version 1.1.',
    copyright: 'Copyright (c) 2020 The Inter Project Authors',
    source: 'folio-designer/public/fonts/inter/Inter-Regular.ttf',
    mediaType: 'font/ttf',
    bytes: new Uint8Array([0x00, 0x01, 0x00, 0x00, 0xff]).buffer,
    tail: ['Noto Sans Thai', 'Noto Sans SC'],
  }

  it('sends twelve fields, the face as base64, and the proposed tail verbatim', () => {
    const command = parsed(embedFontFamilyCommand(face))
    expect(Object.keys(command)).toHaveLength(12)
    expect(command).toEqual({
      kind: 'embedFontFamily',
      version: 1,
      name: 'Inter',
      family: 'Inter',
      style: 'Regular',
      licence: 'OFL-1.1',
      licenceText: face.licenceText,
      copyright: face.copyright,
      source: face.source,
      mediaType: 'font/ttf',
      data: 'AAEAAP8=',
      tail: ['Noto Sans Thai', 'Noto Sans SC'],
    })
    // THE BYTES SURVIVE THE ROUND TRIP, including the high byte. A base64
    // encoder built on String.fromCharCode over a Uint8Array is correct here
    // and silently wrong over a Uint16 view, and 0xff is where that shows.
    expect([...new Uint8Array([...atob(command['data'] as string)].map((character) => character.charCodeAt(0)))]).toEqual([0x00, 0x01, 0x00, 0x00, 0xff])
  })

  it('encodes a face far larger than one call stack, which is the size every real face has', () => {
    // 480 KB is about the largest committed catalogue face. The naive
    // `String.fromCharCode(...bytes)` throws at this size — the spread puts
    // half a million arguments on the stack — so this is the case the chunked
    // encoder exists for, and it is measured rather than reasoned about.
    const large = new Uint8Array(480_000)
    for (let at = 0; at < large.length; at++) large[at] = at % 256
    const command = parsed(embedFontFamilyCommand({ ...face, bytes: large.buffer }))
    const decoded = atob(command['data'] as string)
    expect(decoded.length).toBe(large.length)
    expect(decoded.charCodeAt(0)).toBe(0)
    expect(decoded.charCodeAt(255)).toBe(255)
    expect(decoded.charCodeAt(large.length - 1)).toBe((large.length - 1) % 256)
  })

  it('escapes a licence text through the same encoder every other field uses', () => {
    // A licence is the longest and most punctuated value on this wire — real
    // ones carry newlines, quotes and backslashes — and it is exactly the
    // field where a hand-rolled escape would produce bytes Go cannot parse,
    // losing the located refusal the panel exists to display.
    const awkward = 'Terms "as is",\nwith a \\ and a \u0001 control'
    const command = parsed(embedFontFamilyCommand({ ...face, licenceText: awkward }))
    expect(command['licenceText']).toBe(awkward)
    expect(text(embedFontFamilyCommand({ ...face, licenceText: awkward }))).toContain('\\u0001')
  })

  it('sends an empty tail as an empty array rather than omitting the field', () => {
    // A face that covers every script the document renders needs no fallback
    // behind it — but the field is still counted by componentFields, so
    // omitting it would be an arity refusal rather than an empty chain tail.
    const command = parsed(embedFontFamilyCommand({ ...face, tail: [] }))
    expect(command['tail']).toEqual([])
    expect(Object.keys(command)).toHaveLength(12)
  })
})
