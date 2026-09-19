import { describe, expect, it } from 'vitest'
import { pageSetupCommand } from './page-setup-command'

// STORY 15.2a. This encoder shipped with NO TEST FILE and NO ESCAPING — the
// two facts are related. It is the second front door onto the same defect the
// property encoder had: `preset` and `orientation` were spliced raw INSIDE
// quotes and the six numeric fields raw outside them.
const text = (value: ArrayBuffer): string => new TextDecoder().decode(value)
const margin = { top: '36', right: '36', bottom: '36', left: '36' } as const

describe('pageSetupCommand', () => {
  it('encodes one opaque versioned command with the seven top-level keys Go counts', () => {
    expect(text(pageSetupCommand('A4', 'portrait', '0', '0', margin)))
      .toBe('{"kind":"pageSetup","version":1,"preset":"A4","orientation":"portrait","width":0,"height":0,"margin":{"top":36,"right":36,"bottom":36,"left":36}}')
    expect(text(pageSetupCommand('custom', 'landscape', '595.276', '841.89', { top: '20', right: '18.5', bottom: '20', left: '18.5' })))
      .toBe('{"kind":"pageSetup","version":1,"preset":"custom","orientation":"landscape","width":595.276,"height":841.89,"margin":{"top":20,"right":18.5,"bottom":20,"left":18.5}}')
  })

  it('keeps an emptied draft explicit as null rather than restoring a value nobody typed', () => {
    // The convention was already here in a comment; what it claimed the author
    // would see was wrong, and the Go half of this story fixes the message.
    expect(text(pageSetupCommand('custom', 'portrait', '', '', { top: '', right: '36', bottom: '36', left: '36' })))
      .toBe('{"kind":"pageSetup","version":1,"preset":"custom","orientation":"portrait","width":null,"height":null,"margin":{"top":null,"right":36,"bottom":36,"left":36}}')
  })

  it('cannot be made to carry a second preset, orientation or height from one typed width', () => {
    // DW-73's payload, typed into the width box. Executed at the baseline: this
    // produced valid JSON in which the author's own preset and orientation were
    // both overridden by the last key to appear.
    const payload = text(pageSetupCommand('custom', 'portrait', '0,"preset":"custom","orientation":"landscape","height":9999', '842', margin))
    const command = JSON.parse(payload) as Record<string, unknown>
    expect(command.preset).toBe('custom')
    expect(command.orientation).toBe('portrait')
    expect(command.height).toBe(842)
    expect(command.width).toBe(null)
    expect(Object.keys(command)).toEqual(['kind', 'version', 'preset', 'orientation', 'width', 'height', 'margin'])
  })

  it('quotes preset and orientation, which were spliced raw inside their own quotes', () => {
    // engine-protocol.ts's allowlist is a REMOTE defence on the inbound
    // projection; the check has to sit where its operands are. These two values
    // come from the projection the engine sent, so a projection that ever
    // carried punctuation would have opened a second key here.
    const payload = text(pageSetupCommand('A4","orientation":"landscape', 'portrait', '0', '0', margin))
    const command = JSON.parse(payload) as Record<string, unknown>
    expect(command.preset).toBe('A4","orientation":"landscape')
    expect(command.orientation).toBe('portrait')
    expect(payload.match(/"orientation"/g)).toHaveLength(1)
  })

  it('cannot be made to carry a second margin key from one typed margin', () => {
    const payload = text(pageSetupCommand('A4', 'portrait', '0', '0', { top: '36,"left":9999', right: '36', bottom: '36', left: '36' }))
    const command = JSON.parse(payload) as { margin: Record<string, unknown> }
    expect(Object.keys(command.margin)).toEqual(['top', 'right', 'bottom', 'left'])
    expect(command.margin.top).toBe(null)
    expect(command.margin.left).toBe(36)
  })
})
