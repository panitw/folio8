import { describe, expect, it } from 'vitest'
import { removeSectionBreakCommand, setSectionBreakAnchorCommand, setSectionBreakCommand } from './section-break-command'

// The wire, pinned to the byte and to the key order: Go counts every top-level
// key (componentFields(raw, 4), (raw, 3) and (raw, 2)) and refuses any other arity.
const text = (value: ArrayBuffer): string => new TextDecoder().decode(value)

describe('setSectionBreakCommand', () => {
  it('encodes kind, version, offset and snap, in order', () => {
    expect(text(setSectionBreakCommand('400', true))).toBe('{"kind":"setSectionBreak","version":1,"offset":400,"snap":true}')
    expect(text(setSectionBreakCommand('520.5', false))).toBe('{"kind":"setSectionBreak","version":1,"offset":520.5,"snap":false}')
  })

  it('sends the typed literal untouched, and an emptied or malformed draft as null', () => {
    expect(text(setSectionBreakCommand('1e3', false))).toContain('"offset":1e3')
    expect(text(setSectionBreakCommand('', false))).toContain('"offset":null')
    expect(text(setSectionBreakCommand('40,"snap":true', false))).toBe('{"kind":"setSectionBreak","version":1,"offset":null,"snap":false}')
  })
})

describe('removeSectionBreakCommand', () => {
  it('encodes kind and version only', () => {
    expect(text(removeSectionBreakCommand())).toBe('{"kind":"removeSectionBreak","version":1}')
  })
})

describe('setSectionBreakAnchorCommand', () => {
  it('encodes kind, version and anchor, in order', () => {
    expect(text(setSectionBreakAnchorCommand(false))).toBe('{"kind":"setSectionBreakAnchor","version":1,"anchor":false}')
    expect(text(setSectionBreakAnchorCommand(true))).toBe('{"kind":"setSectionBreakAnchor","version":1,"anchor":true}')
  })
})
