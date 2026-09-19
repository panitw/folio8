// WHERE a font-chain refusal is shown, and why the browser gets to decide it.
//
// The engine's refusal SENTENCE is the engine's, byte for byte (see App.tsx's
// componentDiagnosticDetail and folio-go/wasm/cmd/engine/main.go, which emits
// a *ComponentCommandError as COMPONENT_INVALID with bounded(msg, 512) and
// never through reportableMessage). Its PLACEMENT is browser knowledge and
// nothing else: the panel knows which control it dispatched from, so it does
// not need the refusal's DataPath to put the message beside the right button.
//
// That matters because three chain refusal paths carry NO location at all —
// componentFields' arity refusal, the Canvas(installed) projection-bound
// errors surfaced raw, and serialize/parse failures — and all of them reach
// the browser as ENGINE_REJECTED with no dataPath. Anchoring by ORIGIN handles
// located and unlocated refusals with one rule, which is exactly the shape
// PropertyCommitError already has for property commits.
export type FontChainControl = Readonly<{
  // The chain the command names, as the projection spells it. Absent for the
  // add-chain control, which names a chain that does not exist yet.
  chain?: string
  // The 0-indexed entry position the command names, for the per-entry moves
  // and removes. Absent for whole-chain actions.
  entry?: number
  // A move has TWO controls per entry — earlier and later — and a refusal
  // must light the one that was actually pressed, so the direction is part of
  // the control's identity rather than a detail the anchor loses.
  // `embed` is Story 8.6's pick, and it anchors at the FAMILY CONTROL rather
  // than in the chain editor — that is where the author pressed, and it is
  // also where the disk-font decline is stated, so a refusal and a decline
  // appear in the same place for the same reason.
  action: 'add' | 'rename' | 'delete' | 'addEntry' | 'moveEntryEarlier' | 'moveEntryLater' | 'removeEntry' | 'embed'
}>

// selectionKey mirrors PropertyCommitError's: a refusal that resolves after
// the selection or the document moved on is dropped rather than shown against
// a control the author is no longer looking at. `message` is the engine's own
// string and is rendered verbatim — never prefixed, re-worded or re-ordered.
export type FontChainCommitError = Readonly<{ control: FontChainControl; selectionKey: string; message: string }>

export const sameFontChainControl = (left: FontChainControl, right: FontChainControl): boolean =>
  left.action === right.action && left.chain === right.chain && left.entry === right.entry
