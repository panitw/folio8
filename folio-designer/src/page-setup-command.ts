// The command is deliberately assembled as opaque Go-defined bytes. It is
// not a TypeScript document interface and does not JSON stringify any .folio.
//
// STORY 15.2a: this was the only encoder with no escaping at all. `preset` and
// `orientation` were spliced raw INSIDE quotes and the six numeric fields were
// spliced raw outside them, so one typed width could carry a second `preset`,
// a second `orientation` and a second `height` — a command naming one thing and
// changing several. The allowlist in engine-protocol.ts is a remote defence on
// a different projection; the check belongs where its operands are, and it is
// now the shared authority's.
import { commandBytes, jsonNumber, jsonObject, jsonString } from './command-json'

export function pageSetupCommand(preset: string, orientation: string, width: string, height: string, margin: Readonly<Record<'top' | 'right' | 'bottom' | 'left', string>>): ArrayBuffer {
  // Empty drafts stay explicit (`null`) so Go can return the field-specific
  // diagnostic instead of the UI silently restoring a previous value. This
  // module keeps no rule of its own for that: an empty string is simply not a
  // JSON number, so it falls out of the authority's one shape test alongside
  // every other draft that is not one.
  return commandBytes('pageSetup', [
    ['preset', jsonString(preset)],
    ['orientation', jsonString(orientation)],
    ['width', jsonNumber(width)],
    ['height', jsonNumber(height)],
    ['margin', jsonObject([['top', jsonNumber(margin.top)], ['right', jsonNumber(margin.right)], ['bottom', jsonNumber(margin.bottom)], ['left', jsonNumber(margin.left)]])],
  ])
}
