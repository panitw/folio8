import { registerCarriedFaces } from './embedded-face-registry'
import { previewFaceFamily } from './preview-face-family'

// THE PREVIEW LIFETIME (Story 16.3). THE THIRD ONE IN THIS DESIGNER, AND THE
// ONE GENUINELY NEW MECHANISM IN THIS STORY.
//
// THE TWO THAT ALREADY EXIST, AND WHY NEITHER FITS.
//
//   THE DOCUMENT'S (`App.tsx`, `registerCarriedFaces` keyed on the document
//   generation plus the carried listing). It registers the faces the OPEN
//   DOCUMENT carries and releases them when that document is replaced. A
//   specimen is not carried by any document — the whole point of looking at it
//   is that the author has not decided yet — so this lifetime would either
//   register nothing or register into the document's own namespace.
//
//   THE MACHINE'S (`App.tsx`, keyed on the store's listing). It registers every
//   face this machine already holds, for the session. It cannot cover the web
//   tier at all, which is where 1,273 of the addable families live.
//
// SO THIS ONE IS THE ROW'S: a face is registered when its row is drawn and
// released when that row leaves. Nothing survives the modal closing.
//
// IT MUST NOT BE ABLE TO COLLIDE WITH THE DOCUMENT'S, AND "MUST NOT BE ABLE TO"
// IS STRONGER THAN "DOES NOT". `document.fonts` is a global, name-keyed
// registry. If a preview face landed under a name the canvas asks for, then
// which glyphs the PAGE paints would depend on which page of a MODAL the author
// last looked at — a modal deciding what a document renders as. The two
// namespaces are made disjoint by construction in `preview-face-family.ts`
// (`folio8-preview-` against `folio8-carried-`), and `preview-face-registry.test.ts`
// asserts the disjointness rather than reading it.
//
// ⚠ AND THE GUARD THAT ACTUALLY CATCHES THE HAZARD IS NOT THE ONE THIS COMMENT
// USED TO CREDIT. A red-proof pass deleted the `previewFaceFamily` argument
// below, so preview faces fell back to the DOCUMENT derivation — the exact shape
// of the collision — and the namespace-disjointness test PASSED UNCHANGED,
// because it is a pure-function comparison of two derivations and neither
// derivation changed. What reddened were the behavioural tests that assert which
// family names actually reached the page's font set. Both guards are real, but
// only the font-set observation can see this mistake: do not delete it believing
// the disjointness assertion covers it.
//
// AND IT IS NOT A SECOND AUTHORITY ON WHAT A DOCUMENT CONTAINS. Nothing here
// writes to the machine store, sends a command, touches `fontFamilies` or
// produces anything a `.folio` can carry. A registered preview face is a face
// one `<span>` in one modal can be set in, and that is the entire claim.
//
// THE BOUND IS THE PAGE, AND IT IS WRITTEN DOWN IN `font-browser-model.ts`
// (`familiesPerPage`). This module holds at most one registration per family
// currently shown; `show()` releases every family that is no longer on the list
// it is given, one at a time, so the set held is exactly the set drawn.

/** Resolves the bytes a specimen is set in, or `undefined` when they cannot be had. */
export type PreviewFaceBytes = (family: string) => Promise<ArrayBuffer | undefined>

/**
 * WHAT A ROW MAY SAY ABOUT ITS OWN SPECIMEN. There are exactly three states and
 * the row must render all three DIFFERENTLY, because the contract forbids
 * rendering a specimen in a fallback face while implying it is the family. A
 * row that is `preparing` or `unavailable` says so in words; only `ready` sets
 * the sample in the family.
 */
export type PreviewFaceStatus = 'preparing' | 'ready' | 'unavailable'

export type PreviewFaceRegistry = Readonly<{
  /** Register faces for exactly these families, releasing every other one held. */
  show: (families: ReadonlyArray<string>) => void
  statusOf: (family: string) => PreviewFaceStatus
  /**
   * Release everything.
   *
   * THE MODAL IS NO LONGER THE ONLY CALLER (Story 16.7). The family control
   * opens its own instance of this registry — never the modal's, which is the
   * whole point of `show` releasing every family it is not given — and calls
   * `close()` on ITS OWN unmount, exactly as the modal calls it on its own.
   * The two callers differ in what "the dropdown is open" maps to: the modal
   * IS the open state, so its one open/close is this one `close()`; the
   * family control stays mounted across many opens and closes of its own
   * dropdown, so within one mount those are `show([])` calls, and `close()`
   * only runs when the control itself goes away. Two lifetimes, over the same
   * two operations this module already had.
   */
  close: () => void
}>

type Held = Readonly<{ release: () => void; ready: () => boolean }>

/**
 * `onChange` is called whenever a status moves, so the caller can re-render. It
 * is deliberately a bare notification rather than a payload: the registry is the
 * authority on its own statuses and handing them out in a snapshot would make
 * the caller's copy a second one.
 */
export function openPreviewFaceRegistry(readBytes: PreviewFaceBytes, onChange: () => void): PreviewFaceRegistry {
  let open = true
  const held = new Map<string, Held>()
  // A failure is remembered for as long as the modal is open, so a family whose
  // bytes could not be had does not re-fetch on every page turn back to it. It
  // is a set of NAMES and nothing else — no bytes are cached anywhere, which is
  // what keeps the memory bound the page rather than the session.
  const refused = new Set<string>()

  const decline = (family: string) => {
    if (refused.has(family)) return
    refused.add(family)
    if (open) onChange()
  }

  const registerOne = (family: string): Held => {
    let ready = false
    // `async` so that a resolver throwing SYNCHRONOUSLY becomes a rejection the
    // seam's own catch can see, rather than an exception thrown out of its loop.
    // Every other outcome — no bytes, a name the derivation declines, a face
    // that will not parse — reaches `decline` through the seam's `onDeclined`,
    // which is the single path now that there is one.
    const release = registerCarriedFaces([family], async (name) => readBytes(name), () => { ready = true; if (open) onChange() }, previewFaceFamily, decline)
    return { release, ready: () => ready }
  }

  return {
    show: (families) => {
      if (!open) return
      const wanted = new Set(families)
      for (const [family, entry] of held) {
        if (wanted.has(family)) continue
        entry.release()
        held.delete(family)
      }
      for (const family of wanted) {
        if (held.has(family) || refused.has(family)) continue
        // THE DERIVATION IS CHECKED BEFORE A BYTE IS FETCHED, and both halves of
        // that matter. A name `previewFaceFamily` will not encode can never be
        // registered, so fetching for it spends a full upstream resolution —
        // four metadata probes, a licence file and the face itself, for the web
        // tier — on a specimen that could not have been drawn. And the row must
        // reach `unavailable` rather than sitting on `preparing` for ever, which
        // is what happened while this decision was made after the fetch.
        if (previewFaceFamily(family) === undefined) { decline(family); continue }
        held.set(family, registerOne(family))
      }
    },
    statusOf: (family) => {
      if (refused.has(family)) return 'unavailable'
      return held.get(family)?.ready() === true ? 'ready' : 'preparing'
    },
    close: () => {
      open = false
      for (const entry of held.values()) entry.release()
      held.clear()
      refused.clear()
    },
  }
}
