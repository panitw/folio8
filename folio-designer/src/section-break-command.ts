// spec-section-break CAP-1 / CAP-7. The three Section Break commands, as opaque
// Go-defined bytes.
//
// In the file the break is the content band's `sectionBreak` key, but on the
// canvas it is placed, dragged, nudged, typed and deleted like an element.
// Each of those is ONE command, so each is one undo entry.
//
// THIS MODULE HOLDS NO BOUND AND NO GRID. The engine snaps (`snap` is true for
// a placement and a drag, false for a typed Y and an arrow nudge) and the
// engine refuses a break outside the content band or through an element, with
// a located sentence naming what is in the way.
//
// SPEC-multi-pages story 5: every designed page holds its own break, so each
// command takes an optional 0-based `page`. Page 1 omits it and sends today's
// bytes exactly.
import { commandBytes, jsonBoolean, jsonNumber } from './command-json'

const pageField = (page: number | undefined) => page === undefined || page === 0 ? [] : [['page', jsonNumber(page)] as const]

// `offset` is points, as the author typed it or as `points()` spelled a
// gesture's proposal. jsonNumber sends the literal byte for byte or `null`.
export function setSectionBreakCommand(offset: string, snap: boolean, page?: number): ArrayBuffer {
  return commandBytes('setSectionBreak', [['offset', jsonNumber(offset)], ['snap', jsonBoolean(snap)], ...pageField(page)])
}

export function removeSectionBreakCommand(page?: number): ArrayBuffer {
  return commandBytes('removeSectionBreak', [...pageField(page)])
}

// CAP-7: the Anchor checkbox. One command, so one undo entry; the engine
// writes `sectionBreakAnchor: false` for false and removes the key for true.
export function setSectionBreakAnchorCommand(anchor: boolean, page?: number): ArrayBuffer {
  return commandBytes('setSectionBreakAnchor', [['anchor', jsonBoolean(anchor)], ...pageField(page)])
}
