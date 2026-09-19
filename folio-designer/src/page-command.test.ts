import { describe, expect, it } from 'vitest'
import { addPageCommand, deletePageCommand, setPageBreakCommand } from './page-command'

// The wire, pinned to the byte and to the key order: Go counts every top-level
// key (componentFields) and refuses any other arity.
const text = (value: ArrayBuffer): string => new TextDecoder().decode(value)

describe('addPageCommand', () => {
  it('encodes kind and version alone to append, and after to insert', () => {
    expect(text(addPageCommand())).toBe('{"kind":"addPage","version":1}')
    expect(text(addPageCommand(1))).toBe('{"kind":"addPage","version":1,"after":1}')
    expect(text(addPageCommand(0))).toBe('{"kind":"addPage","version":1,"after":0}')
  })
})

describe('deletePageCommand', () => {
  it('encodes kind, version and page', () => {
    expect(text(deletePageCommand(2))).toBe('{"kind":"deletePage","version":1,"page":2}')
  })
})

describe('setPageBreakCommand', () => {
  it('encodes kind, version, page and pageBreak, in order', () => {
    expect(text(setPageBreakCommand(1, false))).toBe('{"kind":"setPageBreak","version":1,"page":1,"pageBreak":false}')
    expect(text(setPageBreakCommand(3, true))).toBe('{"kind":"setPageBreak","version":1,"page":3,"pageBreak":true}')
  })
})
