import { describe, expect, it } from 'vitest'
import { documentLocaleCommand, documentUTCOffsetCommand } from './document-settings-command'
import { LOCALE_TAGS } from './engine-protocol'

// The wire, pinned to the byte and to the KEY ORDER, in the shape the sibling
// factories' tests use. Order is part of the contract twice over: Go counts
// every top-level key (componentFields(raw, 3)) and refuses any other arity,
// and this module's whole job is to be unable to build anything else.
const text = (value: ArrayBuffer): string => new TextDecoder().decode(value)

describe('documentLocaleCommand', () => {
  it('encodes one opaque versioned command with the three top-level keys Go counts, in order', () => {
    expect(text(documentLocaleCommand('th')))
      .toBe('{"kind":"setDocumentLocale","version":1,"locale":"th"}')
    expect(Object.keys(JSON.parse(text(documentLocaleCommand('th'))) as Record<string, unknown>))
      .toEqual(['kind', 'version', 'locale'])
  })

  it('encodes every tag in the closed set, and spells none of them itself', () => {
    // ENUMERATED FROM LOCALE_TAGS, not written out again: a fifth tag added in
    // Go arrives here through the mirror without an edit, and a factory that
    // could only build one of the four reddens.
    expect(LOCALE_TAGS.length).toBeGreaterThan(0)
    expect(LOCALE_TAGS.map((tag) => text(documentLocaleCommand(tag))))
      .toEqual(LOCALE_TAGS.map((tag) => `{"kind":"setDocumentLocale","version":1,"locale":"${tag}"}`))
    // THE HYPHENATED TAG, DERIVED RATHER THAN SPELLED. It is the one whose
    // spelling a hand-written union gets wrong, and the one that proves the
    // wire carries the tag EXACTLY as the `.folio` file does — never a display
    // name, never a lower-cased variant. It is found by SHAPE (the only tag
    // carrying a script subtag) instead of by literal, because
    // engine-bounds-mirror.test.ts walks all of `src/` for a second spelling of
    // this set and a literal here would be one — which is exactly how that
    // census found this line.
    const scripted = LOCALE_TAGS.filter((tag) => tag.includes('-'))
    expect(scripted).toHaveLength(1)
    const hyphenated = scripted[0] as (typeof LOCALE_TAGS)[number]
    expect(text(documentLocaleCommand(hyphenated))).toContain(`"locale":"${hyphenated}"`)
  })

  it('cannot be made to carry a second locale from one selected value', () => {
    // The splice payload. A raw template literal would have produced valid JSON
    // here in which the command declares `locale` twice — which Go's
    // refuseDuplicateCommandKeys refuses outright, but only after the designer
    // has already built a command whose meaning is decided by last-wins.
    const payload = text(documentLocaleCommand('th","utcOffset":"+09:00' as never))
    const command = JSON.parse(payload) as Record<string, unknown>
    expect(command.locale).toBe('th","utcOffset":"+09:00')
    expect(payload.match(/"utcOffset"/g)).toBeNull()
    expect(Object.keys(command)).toEqual(['kind', 'version', 'locale'])
  })
})

describe('documentUTCOffsetCommand', () => {
  it('encodes one opaque versioned command with the three top-level keys Go counts, in order', () => {
    expect(text(documentUTCOffsetCommand('+07:00')))
      .toBe('{"kind":"setDocumentUTCOffset","version":1,"utcOffset":"+07:00"}')
    expect(Object.keys(JSON.parse(text(documentUTCOffsetCommand('+07:00'))) as Record<string, unknown>))
      .toEqual(['kind', 'version', 'utcOffset'])
  })

  it('sends the author\'s own literal rather than a repair of it', () => {
    // The engine owns what a legal offset IS — ±HH:MM, range-enforced, one
    // predicate shared by the loader and the command door (D-12.C). A factory
    // that padded `+7:00` to `+07:00`, or upper-cased `z`, would be a browser
    // holding an engine rule, and the author would see a value they did not
    // type beside a refusal about one they did.
    expect(text(documentUTCOffsetCommand('+7:00'))).toContain('"utcOffset":"+7:00"')
    expect(text(documentUTCOffsetCommand('+99:99'))).toContain('"utcOffset":"+99:99"')
    expect(text(documentUTCOffsetCommand('Z'))).toContain('"utcOffset":"Z"')
    expect(text(documentUTCOffsetCommand('  +07:00  '))).toContain('"utcOffset":"  +07:00  "')
  })

  it('keeps an emptied draft explicit rather than restoring a value nobody typed', () => {
    // A string field's version of band-height's `null`: the empty box travels
    // as the empty string, which Go's arm refuses by naming the field. What it
    // must never do is silently become the projected value or a default.
    expect(text(documentUTCOffsetCommand('')))
      .toBe('{"kind":"setDocumentUTCOffset","version":1,"utcOffset":""}')
  })

  it('cannot be made to carry a second offset or a second key from one typed value', () => {
    const payload = text(documentUTCOffsetCommand('+07:00","locale":"ja'))
    const command = JSON.parse(payload) as Record<string, unknown>
    expect(command.utcOffset).toBe('+07:00","locale":"ja')
    expect(payload.match(/"utcOffset"/g)).toHaveLength(1)
    expect(payload.match(/"locale"/g)).toBeNull()
    expect(Object.keys(command)).toEqual(['kind', 'version', 'utcOffset'])
  })
})
