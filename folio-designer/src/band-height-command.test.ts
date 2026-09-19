import { describe, expect, it } from 'vitest'
import { bandHeightCommand } from './band-height-command'

// The wire, pinned to the byte and to the KEY ORDER, in the shape the sibling
// factories' tests use. Order is part of the contract twice over: Go counts
// every top-level key (componentFields(raw, 5)) and refuses any other arity,
// and this module's whole job is to be unable to build anything else.
//
// THE ARITY MOVED IN STORY 12.5, FROM FOUR KEYS TO FIVE, and the expectations
// below moved with it deliberately. That is not a violation of 12.1's
// byte-identity criterion: what 12.1 preserved is the DOCUMENT bytes the
// panel's typed path produces, and passing `snap: false` keeps those exactly
// where they were. The COMMAND payload is a different thing and it did change.
const text = (value: ArrayBuffer): string => new TextDecoder().decode(value)

describe('bandHeightCommand', () => {
  it('encodes one opaque versioned command with the five top-level keys Go counts, in order', () => {
    expect(text(bandHeightCommand('pageHeader', '80', false)))
      .toBe('{"kind":"setBandHeight","version":1,"band":"pageHeader","height":80,"snap":false}')
    expect(text(bandHeightCommand('pageFooter', '30.125', false)))
      .toBe('{"kind":"setBandHeight","version":1,"band":"pageFooter","height":30.125,"snap":false}')
    expect(Object.keys(JSON.parse(text(bandHeightCommand('pageHeader', '80', false))) as Record<string, unknown>))
      .toEqual(['kind', 'version', 'band', 'height', 'snap'])
  })

  // THE FIFTH FIELD IS CARRIED, NOT DROPPED. A factory that took the parameter
  // and always emitted `false` would pass every other row in this file: the
  // key would be present, the arity right, the order right, and the canvas
  // boundary drag would silently stop landing on the grid.
  it('sends the two snap states as two different payloads', () => {
    const off = text(bandHeightCommand('pageHeader', '80', false))
    const on = text(bandHeightCommand('pageHeader', '80', true))
    expect(on).toBe('{"kind":"setBandHeight","version":1,"band":"pageHeader","height":80,"snap":true}')
    expect(on).not.toBe(off)
    expect(text(bandHeightCommand('pageFooter', '30.125', true))).toContain('"snap":true')
  })

  it('sends the author\'s own literal rather than a re-computation of it', () => {
    // The engine owns what a legal height IS: three decimal places, no
    // exponent, inside MaxCanvasMillipoints. A factory that ran the draft
    // through Number() would widen that accept-set — `1e3` reaching Go as
    // `1000` is the measured shape of that defect — so the literal travels
    // untouched and Go answers with its own located sentence.
    expect(text(bandHeightCommand('pageHeader', '80.0001', false))).toContain('"height":80.0001')
    expect(text(bandHeightCommand('pageHeader', '1e3', false))).toContain('"height":1e3')
    expect(text(bandHeightCommand('pageHeader', '-5', false))).toContain('"height":-5')
  })

  it('keeps an emptied draft explicit as null rather than restoring a value nobody typed', () => {
    // Number('') is 0 in JavaScript, and silently sending a 0 for a box the
    // author emptied is exactly the invented value the shared authority exists
    // to stop. `null` is the convention page setup already ships.
    expect(text(bandHeightCommand('pageFooter', '', false)))
      .toBe('{"kind":"setBandHeight","version":1,"band":"pageFooter","height":null,"snap":false}')
    expect(text(bandHeightCommand('pageHeader', '  12  ', false))).toContain('"height":null')
  })

  it('cannot be made to carry a second band or a second height from one typed number', () => {
    // The splice payload, typed into the height box. A raw template literal
    // would have produced valid JSON here in which the band the command NAMES
    // is not the band it CHANGES.
    const payload = text(bandHeightCommand('pageHeader', '80,"band":"pageFooter"', false))
    const command = JSON.parse(payload) as Record<string, unknown>
    expect(command.band).toBe('pageHeader')
    expect(command.height).toBe(null)
    expect(payload.match(/"band"/g)).toHaveLength(1)
    expect(Object.keys(command)).toEqual(['kind', 'version', 'band', 'height', 'snap'])
  })
})
