import { describe, expect, it } from 'vitest'
import { bindComponentScalarCommand, bindTableCollectionCommand, createComponentCommand, deleteComponentsCommand, dropComponentCommand, duplicateComponentsCommand, moveComponentCommand, moveComponentsCommand, resizeComponentCommand } from './component-command'

const text = (value: ArrayBuffer) => new TextDecoder().decode(value)

describe('opaque component commands', () => {
  it('converts projection millipoints to exact point literals once for move and resize', () => {
    expect(text(moveComponentCommand('e9', 1001, 2002, false))).toBe('{"kind":"moveComponent","version":1,"id":"e9","x":1.001,"y":2.002,"snap":false}')
    expect(text(resizeComponentCommand('e9', 73003, 25004, true))).toBe('{"kind":"resizeComponent","version":1,"id":"e9","width":73.003,"height":25.004,"snap":true}')
  })

  it('sends a global document point to the Go-owned drop hit test', () => {
    expect(text(dropComponentCommand('text', 36, 56, true))).toBe('{"kind":"dropComponent","version":1,"type":"text","x":36,"y":56,"snap":true}')
  })

  it('encodes decoded picker segments with complete JSON escaping', () => {
    expect(text(bindComponentScalarCommand('e1', ['a.b', 'line\nbreak', '\u0000']))).toBe('{"kind":"bindComponentScalar","version":1,"id":"e1","segments":["a.b","line\\nbreak","\\u0000"]}')
  })

  it('transports collection keys verbatim, leaving grammar and alias ownership to Go', () => {
    const segments = ['a.b', '', 'สวัสดี', 'line\nbreak', '\u0000', '"quoted"', '\\']
    expect(JSON.parse(text(bindTableCollectionCommand('e"8', segments)))).toEqual({ kind: 'bindTableCollection', version: 1, id: 'e"8', segments })
  })
})

it('encodes a captured group movement as one relative, revision-fenced command', () => {
  expect(new TextDecoder().decode(moveComponentsCommand(['e1', 'e2'], 'e2', -1125, 2227, true, 17))).toBe('{"kind":"moveComponents","version":1,"ids":["e1","e2"],"referenceId":"e2","dx":-1.125,"dy":2.227,"snap":true,"expectedRevision":17}')
})

it('encodes a group delete and a group duplicate as one command each', () => {
  expect(new TextDecoder().decode(deleteComponentsCommand(['e1', 'e9']))).toBe('{"kind":"deleteComponents","version":1,"ids":["e1","e9"]}')
  expect(new TextDecoder().decode(duplicateComponentsCommand(['e1', 'e9'], true))).toBe('{"kind":"duplicateComponents","version":1,"ids":["e1","e9"],"snap":true}')
  expect(new TextDecoder().decode(duplicateComponentsCommand(['e1'], false, 2))).toBe('{"kind":"duplicateComponents","version":1,"ids":["e1"],"snap":false,"page":2}')
  expect(JSON.parse(new TextDecoder().decode(deleteComponentsCommand(['e"1'])))).toEqual({ kind: 'deleteComponents', version: 1, ids: ['e"1'] })
})

// Pointer-only policy remains optional for continuous-coordinate callers.
it('encodes the current-window constraint only when requested', () => {
  const decode = (value: ArrayBuffer) => JSON.parse(new TextDecoder().decode(value))
  expect(decode(moveComponentsCommand(['e1'], 'e1', 0, 9000, false, 1))).not.toHaveProperty('constrainToWindow')
  expect(decode(moveComponentsCommand(['e1'], 'e1', 0, 9000, false, 1, true))).toHaveProperty('constrainToWindow', true)
})

// SPEC-multi-pages story 3: the target page is written last, and only when given.
it('writes the target page last and only when given, so page 1 keeps today bytes', () => {
  const decode = (value: ArrayBuffer) => new TextDecoder().decode(value)
  expect(decode(createComponentCommand('text', 'content', 100, 50, true))).toBe('{"kind":"createComponent","version":1,"type":"text","band":"content","x":100,"y":50,"width":72,"height":24,"snap":true}')
  expect(decode(createComponentCommand('text', 'content', 100, 50, true, 1))).toBe('{"kind":"createComponent","version":1,"type":"text","band":"content","x":100,"y":50,"width":72,"height":24,"snap":true,"page":1}')
  expect(decode(dropComponentCommand('text', 36, 56, true))).toBe('{"kind":"dropComponent","version":1,"type":"text","x":36,"y":56,"snap":true}')
  expect(decode(dropComponentCommand('text', 36, 56, true, 2))).toBe('{"kind":"dropComponent","version":1,"type":"text","x":36,"y":56,"snap":true,"page":2}')
  expect(decode(moveComponentsCommand(['e1'], 'e1', 0, 200000, false, 3, true))).toBe('{"kind":"moveComponents","version":1,"ids":["e1"],"referenceId":"e1","dx":0,"dy":200,"snap":false,"expectedRevision":3,"constrainToWindow":true}')
  expect(decode(moveComponentsCommand(['e1'], 'e1', 0, 200000, false, 3, true, 1))).toBe('{"kind":"moveComponents","version":1,"ids":["e1"],"referenceId":"e1","dx":0,"dy":200,"snap":false,"expectedRevision":3,"constrainToWindow":true,"page":1}')
})
