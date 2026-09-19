// STORY 12.1. The band-height command, as opaque Go-defined bytes.
//
// It is a COMPONENT command, not a page-setup one, and that is the whole reason
// this file exists rather than two more keys on pageSetupCommand: the engine's
// page-setup door gates on a seven-key arity that every caller's shape depends
// on, while the component door dispatches on `kind` and counts each arm's own
// fields. The seven font-chain factories are the shipped precedent for a
// document-level mutation encoded here.
//
// THIS MODULE HOLDS NO BOUND OF ITS OWN, and that is a ruling rather than an
// omission (Story 17.4 item 9, restated by 12.1's Q4). A floor at the lowest
// occupied edge of the band is computable from the projection alone — and a
// bound the browser can only derive from LAYOUT does not belong in the
// inspector, however convenient. The panel proposes the number the author
// typed, the engine refuses it with a located sentence, and the existing
// role="alert" path renders that sentence. Consistency with typing is the
// property the tests assert.
//
// STORY 12.5 NARROWS THAT RULING TO THE PANEL. It is a narrowing and not a
// deletion: the typed field above still holds no bound, and the canvas
// boundary drag (band-boundary.ts) holds two — a floor at 0 and the mirrored
// content-window ceiling. THE REASON THE TWO DIFFER, because an asymmetry with
// no reason attached is one the next reader will "fix" in whichever direction
// they happen to prefer:
//
//   - 17.4's asserted property is CONSISTENCY WITH TYPING. The panel field is
//     typed, so it has something to be consistent with: an arrow key that
//     clamped where the keystroke beside it sends-and-is-refused would make
//     one box behave two ways. A canvas boundary has NO typed counterpart, so
//     that property is vacuous there and cannot be violated.
//   - 17.4's other objection is a QUIETLY-DRIFTING copy of the engine's rule.
//     DW-36 answered that half by condition rather than by prohibition: the
//     browser bound must CONSUME the engine's own declaration and
//     engine-bounds-mirror.test.ts must read it doing so. band-boundary.ts's
//     ceiling is inside that census, so it is not a quiet copy.
//
// The discriminator, stated once so it can be reused: CLAMP A GESTURE AT
// BOUNDS THAT CARRY NO INFORMATION; SEND, AND LET THE ENGINE REFUSE, WHERE THE
// REFUSAL NAMES SOMETHING THE AUTHOR NEEDS. A ceiling says only "no further",
// which a stopped pointer already says. The STRAND refusal names the element
// in the way, so it is not clamped in the browser at all — in the panel or on
// the canvas.
import { commandBytes, jsonBoolean, jsonNumber, jsonString } from './command-json'
import type { CappingBand } from './engine-protocol'

// Only the bands that CAP VERTICALLY have a height a command may set. `content`
// is absent by MEANING, not by omission: its height is derived from the page
// and the two bands above and below it, and the engine refuses a command naming
// it.
//
// THE LIST IS NOT SPELLED AGAIN HERE. Go's bandsCappingVertically is mirrored
// in engine-protocol.ts as BANDS_CAPPING_VERTICALLY, and
// engine-bounds-mirror.test.ts reads both sides; a union written out in this
// file would be a fourth copy standing outside that census, which is the only
// place a stale copy of this list can hide. CappingBand is that same list as a
// type.

// `height` is the author's DRAFT, passed as typed. jsonNumber tests it against
// the JSON number grammar and sends it byte for byte or sends `null` — an
// emptied box becomes `null` and the engine names the field, exactly as page
// setup already behaves. Nothing here re-computes it.
//
// `snap` is the fifth field Story 12.5 added, and it is the ENGINE that rounds
// (R3). Every other geometry factory — create, drop, move, resize,
// setComponentBounds, duplicate — already carries it, and SnapNearest's
// exact-halves-away-from-zero rule is written down in exactly one place.
// Rounding here would be the first grid arithmetic in folio-designer and a
// fourth spelling of a rule stated once. The panel passes `false`, so the
// DOCUMENT BYTES its typed path writes are unchanged; the command PAYLOAD is
// not, and deliberately so — it gains `"snap":false`.
export function bandHeightCommand(band: CappingBand, height: string, snap: boolean): ArrayBuffer {
  return commandBytes('setBandHeight', [['band', jsonString(band)], ['height', jsonNumber(height)], ['snap', jsonBoolean(snap)]])
}
