---
title: 'Complete an opened document''s families'
type: 'feature'
created: '2026-09-19'
status: 'draft'
route: 'dispatch'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** A document saved before this spec carries one face per family. Its
author presses **B** and is told the family has no bold, even where the family
publishes one — because nothing ever fetched it onto this machine. SPEC-install-all-face-cuts
CAP-4 closes that: opening such a document offers to fetch the missing cuts and, on
acceptance, fills the designer's local face store in the background.

**Approach:** After `load` has projected the document, derive the families whose cuts
this machine is short of from `canvas.fontChains`, ask the author once, and on
acceptance fetch the missing cuts and write them to the machine store — un-awaited, so
the open never waits, with progress and then an outcome shown while the author keeps
editing. Nothing reaches the document.

Planned against stories **1** (a family's whole face set is fetchable and storable, the
store is modelled as a face *set* per family) and **2** (a cut is embedded on first use;
the absence sentence consults the store) having landed. Planned against the **working
tree as it stands on disk**, which carries ~25 files of uncommitted
spec-deferred-offline-cache work, including a modified `document-face-prefetch.ts` and a
new, uncommitted `absent-face-recovery.ts`.

## Boundaries & Constraints

**Always:**
- **The document is not touched.** No engine `command`, `undo` or `redo` request, no
  `setCurrentSnapshot`, no `setBaselineRevision`, no `documentGeneration` bump, no save.
  Completion's only sinks are the machine store (`keepOnThisMachine`) and, if OQ-2 puts
  the catalogue tier in scope, the release's service-worker cache
  (`refreshHeldLocalFamilies`). If the work reaches for a document command, stop and ask.
- **The open does not wait.** The completion call sits *after* `setCurrentSnapshot` in
  `installOpenedDocument` and is not awaited. The canvas paints and the document is
  editable before any completion request resolves.
- **The author is asked; completion never runs silently.** Per SPEC.md Assumptions, a
  decline is not remembered — the next open of the same document asks again.
- **Fetches are eager once accepted** — all missing cuts for the document's families, at
  open, not lazily on the first **B**.
- **Per-face failure is not family failure.** A refused or unreachable cut leaves the
  family's other cuts installed and is reported as a shortfall, never as a throw.
- **Nothing here throws into the open path.** Every failure ends as a sentence.
- **A result that arrives after the document was replaced is not shown.** Capture
  `documentGeneration.current` before the first await and gate every UI write on it
  (the `applyImageAsset` pattern, `App.tsx:2947-2960`). Store writes are
  document-independent and are kept regardless.

**Never:**
- Never parse the `.folio` in TypeScript. The families come from the engine's projection
  (`canvas.fontChains`), which is the only source the designer has (AD-17).
- Never embed, declare a chain entry, or widen `embedFontFamily`. That is story 2's wire
  and this story does not touch it.
- Never change `prefetchDeferredFaces`' own awaited call at `App.tsx:3118`, its
  painted-face selection (`deferredFaceAssets` / `paintedCanvasFaces`), or
  `absent-face-recovery.ts`. Reuse the transport only.
- Never prompt where nothing can be kept: a browser with no usable store
  (`storeKeepsFaces === false`) is offered nothing.
- Never prompt for the starter or a blank document. Neither routes through
  `installOpenedDocument`, and their families are shipped faces that already carry cuts.
- No synthetic cuts, no variable faces, no weight beyond the four the format declares.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Nothing to complete | every document family's cuts already held | no prompt, no request | N/A |
| Shipped-only document | chains name only shipped faces | no prompt | N/A |
| No store | `storeKeepsFaces === false` | no prompt | N/A |
| Incomplete family | store holds Regular only; family publishes more | prompt naming the families; document editable while it stands | N/A |
| Decline | author declines | prompt clears; nothing fetched; revision, `canUndo` and bytes unchanged; next open asks again | N/A |
| Accept | author accepts | progress with a numeric readout, then a settled outcome sentence; store gains the cuts | per-cut failures counted |
| Offline | `navigator.onLine === false` at accept | one outcome sentence saying the cuts could not be fetched; no request made | no throw |
| One cut refused | italic is a variable font, or a non-`ttf`/`otf` media type | the family's other cuts still land; outcome names the shortfall | per-face refusal, family kept |
| Document replaced mid-flight | author opens another document | no completion status lands on the new document; store keeps what arrived | generation guard |

</frozen-after-approval>

## Open Questions

1. **Where the author is asked, and where progress is shown.** SPEC.md CAP-4 says *"the
   status bar reports progress and then the outcome"*. The literal status bar is the
   24 px `<footer className="status-bar">` (`App.tsx:4050`) — derived spans only, no
   message API — and in **design** mode it already carries `LOCAL SHELL`, the engine
   snapshot, the font count, the bound-element count, `offline-status` (whose longest
   label is 48 characters) and `DESIGN MODE`. `.app-shell` is `overflow: hidden`, so an
   overfull bar is clipped rather than wrapped. The other message slot, `fileStatus`
   (`App.tsx:431`, rendered `:3837`), is what the open itself writes (*"Opened local file
   X"*) and auto-retires after 6 s — a background completion writing there would stomp
   the open's own outcome.
   - options: **(A) the `status-bar` footer** — literally what SPEC.md says; needs a new
     span and a real width measurement at 1024 px before it can be trusted, and design
     mode is the crowded one / **(B) the `fileStatus` message bar** — an existing setter
     with an existing transient rule, but it collides with the open's own sentence and
     with `fileBusy` / **(C) a non-modal region beside the canvas**, in the shape of the
     canvas face-miss list (`App.tsx:3907`, `role="status" aria-live="polite"` with
     per-item actions) — carries the question, the numeric progress and the outcome, has
     no width budget, and leaves the document editable throughout, but is not the
     "status bar" SPEC.md names / **(D) a modal dialog** in the `UnsavedChangesDialog`
     shape (`App.tsx:6066`) — the established yes/no vocabulary, but the document is not
     editable until it is answered, which reads against CAP-4's *"editable throughout"*.
   - My read: **(C)** for the prompt and progress. DESIGN.md:579-581 and :599 require
     *"Always paired with a numeric readout"*; EXPERIENCE.md:72-74 requires progress
     indication rather than a spinner. (C) satisfies both without a geometry risk. If the
     owner wants the literal status bar, (A) is buildable but adds an e2e width
     measurement to this story.

2. **Which tiers completion covers.** A document's families can be author-installed web
   faces (machine store, `font-store.ts`) or committed-catalogue faces (release assets
   behind the service worker, `held-local-faces.ts`). The two have different sources,
   different sinks and different code (`installFamily`'s two arms, `App.tsx:2694-2713`
   and `:2717-2773`). Catalogue families have no cuts at all until **story 3** ships them.
   - options: **(A) web-installed families only** — depends on stories 1 and 2 as
     dispatched, ships in isolation, but an offline author with a catalogue family gets
     nothing, which reads against CAP-3's *"No offline author loses bold"* / **(B) both
     tiers** — matches CAP-3 and CAP-4 read together, but adds story 3 as a hard
     dependency and roughly doubles this story's footprint / **(C) both tiers, with the
     catalogue arm written but inert until story 3's catalogue rows carry a style** —
     one story, ordering-independent, at the cost of an arm nothing exercises yet.
   - My read: **(B)**, and re-dispatch this story after story 3. CAP-4's success text is
     about *"the existing Sarabun document"*, a web family — so (A) delivers the named
     scenario — but shipping completion that silently skips half the families the dialog
     offers is the kind of gap that gets found by an author, not by a test.

3. **How the designer learns what a family publishes.** To say "Sarabun is missing its
   bold", the designer needs Sarabun's upstream cut set. For a web family that lives in
   `METADATA.pb` and costs a network round trip per family; `fetchWebFamily`
   (`font-source.ts:403`) fetches it, but as part of fetching a face.
   - options: **(A) probe upstream before asking** — the prompt is truthful and never
     appears for a family that has nothing to add, but it puts network requests before
     the author has consented, and it cannot run offline, so an offline open would show
     no prompt at all / **(B) ask on store shortfall alone** — no network before consent,
     but the prompt can appear for a Regular-only family and "complete" then completes
     nothing, which is the same untruthful sentence CAP-2 exists to remove / **(C) take
     the cut set from story 5's `styles` carry-through** (`font-index.json` already
     carries `["400","400i"]`; `build-font-index.mjs:124` discards it) — no network, no
     untruthful prompt, but adds story 5 as a dependency and the snapshot is dated.
   - My read: **(C)** if story 5 can precede this, otherwise **(B)** with the prompt
     worded as an offer to *check for* missing cuts rather than a claim that they exist.
     (A) is the one I would not take: a fetch before consent is the thing CAP-4's prompt
     exists to prevent.

## Code Map

- `folio-designer/src/App.tsx:3075` `installOpenedDocument` — the one document-replacement
  path for template bytes (Open + startup-dialog example). Insert the completion call
  after `setCurrentSnapshot` (`:3120`), **un-awaited**. `startBlank` (`:3261`) and the
  launch starter (`startup-sequence.ts:11`) do not route here and stay exempt.
- `folio-designer/src/App.tsx:3118` `await prefetchDeferredFaces(...)` — **do not move or
  unawait.** The await is argued at `:3096-3101`: the browser's `@font-face` bytes must
  land before the canvas paints. Completion is a different job with a different bound.
- `folio-designer/src/engine-protocol.ts:518` `CanvasProjection.fontChains` — entries are
  `{face, assetKey, family, style, bold, italic, boldItalic}`; exactly one of
  `face`/`assetKey` is non-empty. `assetKey` non-empty ⇒ the document carries the face ⇒
  `entry.family` is a family completion may consider. `face` non-empty ⇒ a shipped face,
  out of scope. Discriminant precedent: `App.tsx:888` (`carriedFaceKeys`).
- `folio-designer/src/App.tsx:2926` `keepOnThisMachine(face)` — the store write plus
  `refreshStoredFaces`. Reuse verbatim; it touches no document.
- `folio-designer/src/App.tsx:2672` `installFamily` — the two tier arms completion mirrors:
  catalogue = `fetch(source.face.url)` + `refreshHeldLocalFamilies` (`:2694-2713`, no store
  write, by design); web = `fetchWebFamily` + `keepOnThisMachine` (`:2718-2775`).
- `folio-designer/src/font-source.ts:403` `fetchWebFamily(family, fetcher, today)` —
  injected fetcher, 30 s timeout (`:313`), no progress callback. Story 1 changes it to
  return a face **set**; build on that, do not re-narrow. Per-face rules that stay
  per-face: variable refusal `:513-515`, media type `:206-211`.
- `folio-designer/src/font-store.ts:156` `FontStore.list()` → `StoredFace[]` carrying
  `family` and a non-empty `style` (required at `:234-235`). After story 1 this is the
  authority on which cuts this machine holds. App state: `storedFaces` (`App.tsx:672`),
  `storeKeepsFaces` (`:676`), `heldLocalFamilies` (`:685`).
- `folio-designer/src/document-face-prefetch.ts:22-30` — the comment claiming the open
  fetches **only** faces the document's text painted. This story widens that bound (a cut
  the author has not used is by definition unpainted). Rewrite the comment; do not leave it
  standing over code it no longer describes.
- `folio-designer/src/document-face-prefetch.ts:162` `fetchDeferredFaces(urls, timeoutMs,
  request)` — **uncommitted, new.** Never throws, returns bodies by URL, short-circuits on
  `navigator.onLine === false` (`:165`). This is the reusable transport if OQ-2 puts the
  catalogue tier in scope. Reuse it; do not copy it.
- `folio-designer/src/absent-face-recovery.ts` — **uncommitted, new.** Different problem:
  reactive, engine-FontSet-side, driven by a `TEXT_FACE_ABSENT` refusal, installing release
  face bytes into wasm via `install-face`. It never writes the machine store and its
  candidate set (`generated/canvas-face-assets.ts`) has no row for an author-installed web
  family. **Do not modify it and do not route completion through it.**
- `folio-designer/src/App.tsx:3907` — the canvas face-miss `<ul role="status">` with
  per-item actions: the non-modal precedent for OQ-1 option (C). `App.tsx:6066`
  `UnsavedChangesDialog` is the modal precedent for (D). `FontBrowser.tsx:76`/`:358` is the
  existing `"added N of M"` progress shape.
- `folio-designer/src/App.tsx:127` `SETTLED_FILE_STATUS_MS`, `:2986-2990` — the
  settled-status auto-retire rule; a *busy* status is deliberately exempt (`:110-126`).
- **Do not change:** `engine-protocol.ts` operations, `font-chain-command.ts`,
  `shipped-face-cuts.ts`, `component_commands.go`, anything in `folio-go`.

## Tasks & Acceptance

**Execution:**
- [ ] `folio-designer/src/document-face-completion.ts` -- NEW. Pure model, no React and no
      `fetch`: `incompleteFamilies(fontChains, storedFaces, heldLocalFamilies, publishes)`
      → the families and their missing cuts, plus this surface's sentences (the question,
      the `n of m` progress line, the settled outcome, the offline outcome). Sentences live
      here, not inline in the component, per the repo's per-surface model-module convention
      (`font-browser-model.ts`, `preview/freshness.ts`).
- [ ] `folio-designer/src/App.tsx` -- call the model after `setCurrentSnapshot` in
      `installOpenedDocument`, un-awaited; hold the prompt and the progress in state; on
      accept, fetch per family and write through `keepOnThisMachine` (and
      `refreshHeldLocalFamilies` if OQ-2 includes the catalogue tier); render on the
      surface OQ-1 selects; guard every UI write on `documentGeneration.current`.
- [ ] `folio-designer/src/document-face-prefetch.ts` -- rewrite the `:22-30` bound comment
      so it states the open's two fetches and their different bounds. No behaviour change.
- [ ] `folio-designer/src/App.css` -- styling for the new region, only if OQ-1 needs one.
- [ ] `folio-designer/src/document-face-completion.test.ts` -- NEW. One test per I/O matrix
      row that the pure model can carry.
- [ ] `folio-designer/src/App.test.tsx` -- the rows that need the App: the non-mutation
      proof, the open-not-blocked proof, the generation guard, the offline outcome, and
      the no-store case. Use `withMachineStore()` (`App.test.tsx:37-44`) and the existing
      `globalThis.fetch` save/restore pattern.

**Acceptance Criteria:**
- Given a completion that runs to success, when it settles, then no `command`, `undo` or
  `redo` engine request was issued during it, and `snapshot.revision`, `canUndo` and
  `canRedo` are identical to their values immediately after the open.
- Given a document needing completion, when it opens, then the canvas has rendered and an
  edit is accepted **before** any completion request resolves.
- Given a document that has been completed and re-opened without the cuts being embedded,
  when it opens, then the author is asked again (a decline is not remembered).
- Given a browser where the font store cannot be opened, when such a document opens, then
  no question is asked and no completion request is made.

## Implementation Notes

## Spec Change Log

## Review Triage Log

## Design Notes

**Why the dispatch note's "build on the existing path" is only half-applicable.** There is
one existing open-time face fetch — `prefetchDeferredFaces` at `App.tsx:3118` — and it
differs from completion on all three axes that matter:

| | existing prefetch | completion |
|---|---|---|
| selects | faces the text **painted** (`textPaint…fragments[].face`) | cuts the text has **not** painted, by definition |
| candidate set | `generated/canvas-face-assets.ts` — this release's own shipped + catalogue faces | Google Fonts upstream, per family |
| sink | service-worker cache (browser `@font-face`) | IndexedDB machine store (`font-store.ts`) |
| timing | **awaited**, deliberately (`App.tsx:3096-3101`) | must not be awaited (CAP-4) |

So the reuse is the **transport** (`fetchDeferredFaces`, and only if OQ-2 puts the
catalogue tier in scope) and the tier arms of `installFamily` — not the call site and not
the selection. That is one mechanism widened, not a second mechanism beside it.

## Verification

**Commands:**
- `cd folio-designer && npm run typecheck` -- expected: clean. `tsc -b` builds
  `tsconfig.app.json`, which includes `src`, so the new tests are type-checked too.
- `cd folio-designer && npm run lint` -- expected: clean apart from the 4 pre-existing
  `only-export-components` warnings.
- `cd folio-designer && npm test` -- expected: all pass; report the pass count and diff the
  test-name set against the pre-change run, not just the total.
- `cd folio-designer && npx vitest run src/document-face-completion.test.ts src/document-face-prefetch.test.ts src/App.font-store.test.tsx` -- expected: all pass.

**Manual checks (if no CLI):**
- Red-proof the non-mutation AC by deleting the guard it rests on (issue a `command`
  during completion) and confirming that AC turns red. A green suite over correct code
  proves nothing about whether the assertion can see the defect.
