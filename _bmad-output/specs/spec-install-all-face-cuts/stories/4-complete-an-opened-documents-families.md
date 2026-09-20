---
title: 'Complete an opened document''s families'
type: 'feature'
created: '2026-09-19'
status: 'done'
route: 'dispatch'
baseline_commit: '6dd0997aa05a39aa5f22f41605e1281f99a25894'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** A document saved before this spec carries one face per family, and a document
naming a committed-catalogue family may hold only some of that family's cuts in this
release's cache. Either way the author presses **B** and is told the cut is not on this
machine — and nothing ever fetches it. SPEC-install-all-face-cuts CAP-4 closes that.

**Approach:** After `load` has projected the document, ask `familyIsComplete` of every
family the document's chains name; if any is short, ask the author once, and on acceptance
fetch the missing cuts — un-awaited, so the open never waits — with progress and then an
outcome, while the author keeps editing. Nothing reaches the document.

Re-planned 2026-09-20 against a **clean tree at `6dd0997`**, after stories 1, 6, 2 and 3
shipped. The earlier plan was written against uncommitted work and is superseded.

**Settled by the owner, 2026-09-20:**
- *The ask is a **modal**, in the `UnsavedChangesDialog` shape. Progress and outcome go to the
  literal `status-bar` footer.* The question and the reporting are deliberately split.
- *`offline-status` is hidden in **design** mode **only while a completion line stands**.* A real
  measurement at 1024×768 found the design bar has **136.00 px** of worst-case slack and **clips
  rather than wraps**; the label is in flow in design mode and swings **+210.00 px**. Hiding it
  frees 300 px exactly when the bar needs it, and design keeps its visible offline indicator —
  including `Offline cache unavailable` — the rest of the time. The label is hidden **visually
  only** and stays in the accessibility tree exactly as it is in preview. *The owner first ruled
  a permanent hide, then refined it after review priced what that cost.* It is conditioned on the
  LINE, not on the fetch: an outcome stands for its retire window after the run ends, and a
  run-flag would put the longest sentence beside a visible label — the one combination that does
  not fit.
- *CAP-4's "editable throughout" is amended.* The open never blocks; the author is asked in a
  modal before editing continues; once answered the document is editable throughout the
  **fetching**. The non-blocking requirement was always protecting the fetch, not the question.
  The owner chose the modal knowing it momentarily blocks. SPEC.md CAP-4 carries the amendment.
- *Both tiers are completed, and a local family all-or-nothing.*

## Boundaries & Constraints

**Always:**
- **The document is not touched.** No engine `command`, `undo` or `redo` request, no
  `setCurrentSnapshot`, no `setBaselineRevision`, no `documentGeneration` bump, no save.
  Completion's only sinks are the machine store (`keepOnThisMachine`, `App.tsx:3265`) and
  the release cache (`refreshHeldLocalFamilies`, `App.tsx:711`). Reaching for a document
  command is the signal to stop and ask.
- **The open does not wait.** The call sits after `setCurrentSnapshot` (`App.tsx:3500`) and
  is not awaited. The canvas paints and an edit is accepted before any fetch resolves.
- **Selection is `familyIsComplete`, never `familyIsInstalled`.** `font-index.ts:498`
  answers "is anything left to fetch"; `font-index.ts:449` answers "can these bytes be
  used". Story 3 separated them structurally (`font-index.ts:418-448`, `:475-497`). Do not
  re-fuse them and do not write a third predicate.
- **Reuse the four-state absence vocabulary.** `CutAbsence = 'unpublished' | 'unfetched' |
  'unusable' | 'unchecked'` (`App.tsx:5857`) and `cutAbsenceSentence` (`App.tsx:6068`).
  Completion adds no fifth state and no parallel sentence set.
- **The census is the authority on what a family publishes.** `FamilyCensus`
  (`font-store.ts:201`) plus `censusIsComplete` (`font-store.ts:230`). No network probe
  before the author has consented.
- **A local family is completed all-or-nothing, and the premise is defended on purpose.**
  `cutEmbedPlan` gates the local arm on `completeLocalFamilies` (`App.tsx:5769`), and the
  comment above it (`App.tsx:5749-5756`) argues the partial state is *unreachable* because
  `installFamily` fetches a family's cuts as a set. **Completion is a second producer of
  that state.** Keep the premise true by refreshing local holdings only when every cut of
  that family landed — and **update that comment to name completion as the second producer
  and to say the property survives because completion is deliberately all-or-nothing.** A
  premise that stays true only because nobody noticed the second producer is the shape of
  the D-16.5 misattribution. If all-or-nothing proves impossible rather than awkward — a
  permanently unavailable cut making the whole family uncompletable — **stop and ask**; do
  not widen story 3's gate.
- **The author is asked; completion never runs silently.** A decline is not remembered.
- **Fetches are eager once accepted.**
- **Per-face failure is not family failure**, and nothing here throws into the open path.
- **A result arriving after the document was replaced is not shown.** Capture
  `documentGeneration.current` before the first await (`applyImageAsset`'s pattern,
  `App.tsx:3331`). Store writes are document-independent and are kept regardless.

**Never:**
- Never parse the `.folio` in TypeScript. Families come from `canvas.fontChains` (AD-17).
- Never embed, declare a chain entry, or touch `embedFontFamily` / `cutEmbedPlan`. Story 2
  and story 3 own first-use embedding.
- Never change `prefetchDeferredFaces`' awaited call (`App.tsx:3498`) or its painted-face
  selection. Never modify `absent-face-recovery.ts`.
- Never write to `fileStatus` (`App.tsx:437`): the open writes there and it auto-retires
  after 6 s, so completion would stomp the open's own outcome.
- Never drop, abbreviate past meaning, or allow the clipping of the numeric readout. The
  design-mode budget after the `.sr-only` change is ~424 px; price any new string against
  it at 6.00 px per character before adding it.
- Never remove `offline-status` from the tree or change its text — `.sr-only` is a visual
  hide, and `App.css:7` is `position: absolute`, not `display: none`.
- Never prompt where nothing can be kept (`storeKeepsFaces === false`), and never for the
  starter or a blank document — neither routes through `installOpenedDocument`.
- No synthetic cuts, no variable faces, no weight beyond the format's four.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Nothing to complete | every document family `familyIsComplete` | no prompt, no request | N/A |
| Shipped-only document | chains name only shipped faces | no prompt | N/A |
| No store | `storeKeepsFaces === false` | no prompt | N/A |
| Stored family short a cut | census `published` ⊅ held styles, refusal not permanent | prompt; document editable throughout | N/A |
| Pre-story-1 family | face records, no census row (`unchecked`) | counted as incomplete; completion establishes the census | N/A |
| Catalogue family short a cut | family absent from `LocalFaceHoldings.complete` | prompt; cuts fetched from the deferred release assets | N/A |
| Decline | author declines | prompt clears; nothing fetched; revision, `canUndo` and bytes unchanged; next open asks again | N/A |
| Accept | author accepts | numeric progress, then a settled outcome; store and/or release cache gain the cuts | per-cut failures counted |
| Offline | `navigator.onLine === false` at accept | one outcome saying the cuts could not be fetched; no request | no throw |
| Permanently refused cut | census refusal `permanence: 'permanent'` | not offered, not fetched — `censusIsComplete` already treats it as settled | N/A |
| Partial local landing | 2 of 3 catalogue cuts fetched | local holdings NOT refreshed for that family | see Boundaries |
| Document replaced mid-flight | author opens another document | no completion status lands on the new document; store keeps what arrived | generation guard |

</frozen-after-approval>

## Code Map

Anchors re-derived at `6dd0997`; the earlier map was dated by four shipped stories.

- `App.tsx:3449` `installOpenedDocument` — `load` `:3452`, `serialize` `:3453`,
  `prefetchDeferredFaces` `:3498` (**still awaited — leave it**), `installDocumentIdentity`
  `:3499`, `setCurrentSnapshot` `:3500`. Callers `:3439` (Open), `:3600` (example).
  Insert completion after `:3500`, un-awaited.
- `engine-protocol.ts:518` `CanvasProjection.fontChains` — entries carry
  `{face, assetKey, family, style, bold, italic, boldItalic}`; exactly one of
  `face`/`assetKey` is non-empty. Filter on the discriminant, never the string's shape
  (`App.tsx:910` says why).
- `font-store.ts:201` `FamilyCensus {family, published, refused, recordedAt}`; `:170`
  `FaceCutPermanence = 'permanent' | 'transient'`; `:173` `FamilyCutRefusal`; `:230`
  `censusIsComplete(census, heldCuts)`; store methods `listCensus` `:284` / `putCensus`
  `:286`. App state `familyCensuses` `App.tsx:685`, refreshed with faces in one
  `Promise.all` at `App.tsx:1066-1068`.
- `font-index.ts:498` `familyIsComplete(source, holdings)` — **the selection predicate.**
  `:449` `familyIsInstalled` is the other question; do not substitute it.
- `held-local-faces.ts:92` `LocalFaceHoldings {usable, complete}`; `:163`
  `readLocalFaceHoldings`; `:205` `localFaceIsHeld(url, releaseId)` — the per-cut probe
  story 3 added, and the one this story needs for the local arm.
- `generated/font-catalogue.ts:112` — rows now carry a per-face `style`; **107 faces**
  across the same 31 families (31 Regular, 30 Bold, 23 Italic, 23 BoldItalic). `family`
  stays the base family. This is the local tier's own census; no network needed to know
  what a committed family publishes.
- `App.tsx:2744` `installFamily` — the two arms completion mirrors. Web: `fetchWebFamily`
  with `skip` = held styles (`App.tsx:2812`, `:2822`) → `keepOnThisMachine` (`:3265`) →
  `recordFamilyCensus` (`:3301`). Local: `localFaceIsHeld` → `fetch(face.url)` →
  `refreshHeldLocalFamilies` (`:711`).
- `App.tsx:5857` `CutAbsence`; `:5859` `cutAbsenceState`; `:6068` `cutAbsenceSentence` —
  the four sentences. Reuse; add nothing.
- `App.tsx:5763` `cutEmbedPlan`, gate at `:5769`, premise comment `:5749-5756` — **read
  before touching the local arm.** It declares the partial-cache state unreachable.
- `App.tsx:4430` + `:4441` — the `<footer className="status-bar">`, seven design-mode flex
  items, **no message API**. `offlineLabel` `App.tsx:3981` (five literals, longest 48
  chars). `.status-bar` `App.css:1081` (gap 12, padding 0 12, mono 10px, nowrap);
  `.app-shell` `App.css:19` (`min-width: 1024px`, `overflow: hidden`).
- `App.tsx:3331` — the `requestGeneration` capture pattern for a long await.
- **Do not change:** `document-face-prefetch.ts` behaviour, `absent-face-recovery.ts`,
  `font-chain-command.ts`, anything in `folio-go`.
- **Browser runs:** `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH` must point at chromium **1217**
  (1208 is a 428 KB stub). `e2e/` files are scanned for prohibited identifiers
  (`canvas-authority-contract.test.ts:18`) — no `getComputedStyle`, `getBoundingClientRect`,
  `offset*`, `client*`, `scroll*`. Measure with `boundingBox()` only.

## Tasks & Acceptance

**Execution:**
- [x] `folio-designer/src/document-face-completion.ts` -- NEW. Pure model, no React and no
      `fetch`. `documentFamilies(fontChains)` → the families named by entries whose
      `assetKey` is non-empty, deduped, in chain order (BOTH tiers arrive this way — a
      catalogue family is embedded too). `incompleteDocumentFamilies(fontChains, sources,
      holdings)` → those of them that `offeredFamilies` knows and `familyIsComplete` says
      are short. Plus this surface's sentences: the modal's question and its two button
      labels, the progress line, the settled outcome, the shortfall outcome, the offline
      outcome. Sentences live here, per the repo's per-surface model-module convention.
- [x] `folio-designer/src/App.tsx` -- the status bar gains its FIRST message API: a
      `completionStatus` state and one new `<span data-testid="font-completion-status">`
      in the `<footer className="status-bar">`, rendered only when set. It is NOT
      `fileStatus` and must not auto-retire while work is in flight.
- [x] `folio-designer/src/App.tsx` -- `offline-status` becomes `className="sr-only"` in
      BOTH modes (it is currently `mode === 'preview' ? 'sr-only' : undefined`). This is
      the owner's make-room decision: it frees 288 px plus a 12 px gap in design mode.
      Write the reason beside it — the preview precedent, the measured 136 px worst-case
      slack, and that `App.css:7` is `position: absolute`, so the label is hidden visually
      and stays in the accessibility tree exactly as it is in preview.
- [x] `folio-designer/src/App.tsx` -- a `CompleteFontsDialog` in the
      `UnsavedChangesDialog` shape (`:6893`): `.page-dialog-backdrop` + `.page-dialog`,
      two buttons, the SAFE one focused first and bound to Escape, Tab toggling between
      them, `event.stopPropagation()` so keys never reach the canvas. In-app, never
      `window.confirm`.
- [x] `folio-designer/src/App.tsx` -- in `installOpenedDocument`, AFTER `setCurrentSnapshot`
      (`:3500`): if the store can keep faces and the model reports incomplete families,
      open the dialog. On confirm, call the completion un-awaited. Capture
      `documentGeneration.current` before the first await and gate every UI write on it.
- [x] `folio-designer/src/App.tsx` -- the two tier arms, mirroring `installFamily`
      (`:2744`). Web: `fetchWebFamily(family, …)` with `skip` = the styles already held →
      `keepOnThisMachine` → `recordFamilyCensus`. Local: for each missing cut,
      `catalogueCutOf` → `localFaceIsHeld` → `fetch(face.url)` (body dropped) → after the
      family's whole set, `refreshHeldLocalFamilies()`. A family counts completed only if
      EVERY missing cut landed.
- [x] `folio-designer/src/App.tsx:5749-5756` -- update the premise comment. It currently
      argues the partial local state is unreachable because `installFamily` fetches a
      family's cuts as a set. Name completion as the SECOND producer, and state why the
      property still holds: `readLocalFaceHoldings` counts a family `complete` only when
      every declared cut is held (`held-local-faces.ts:185`), so a partial landing leaves
      the family exactly where it was — still `unfetched`, still offered — and no
      half-complete family is ever treated as complete.
- [x] `folio-designer/src/document-face-completion.test.ts` -- NEW. One test per I/O matrix
      row the pure model can carry.
- [x] `folio-designer/src/App.test.tsx` -- the rows needing the App: the non-mutation
      proof, the open-not-blocked proof, the generation guard, the offline outcome, the
      no-store case, the decline, and the partial-local-landing row. Use
      `withMachineStore()` (`:37-44`) and the existing `globalThis.fetch` save/restore.

**Acceptance Criteria:**
- Given a completion that runs to success, when it settles, then no `command`, `undo` or
  `redo` engine request was issued during it, and `snapshot.revision`, `canUndo` and
  `canRedo` are identical to their values immediately after the open.
- Given a document needing completion, when it opens, then the open resolves and the
  canvas has rendered before the dialog is answered; and once answered, an edit is
  accepted before any completion request resolves.
- Given a completed document re-opened with the cuts still unembedded, when it opens, then
  the author is asked again.
- Given a browser whose font store cannot be opened, when such a document opens, then no
  question is asked and no completion request is made.
- Given design mode with no completion line, then `offline-status` is painted; given a completion
  line standing, then it carries `sr-only`; and in both states, and in preview, it is present in
  the accessibility tree by role and accessible name with its text unchanged.

## Implementation Notes

### THE WEB TIER IS COMPLETED TOO, AND THE ARM WAS ALREADY WRITTEN THAT WAY

The Code Map's web arm is `installFamily`'s web arm, and `installFamily` shares ONE arm between the
`web` and `stored` tiers — the only difference is `held`, which is the stored face records' styles
for a `stored` row and `[]` for a `web` one. `completeOneFamily` mirrors that exactly rather than
restricting itself to `stored`, and it has to: `familyIsComplete` answers `false` for every `web`
row, so `incompleteDocumentFamilies` selects them, and a document carrying a family this machine
holds NOTHING of — one saved before this spec, or opened on another machine — is the plainest
reading of the problem CAP-4 names. Refusing them would have selected a family and then silently
done nothing for it.

### THE CENSUS IS WRITTEN EVEN WHEN A CUT DID NOT LAND — the opposite of `installFamily`

`installFamily` refuses BEFORE `recordFamilyCensus` when a face write fails, because there the write
IS the act. Completion writes it anyway, and the reasoning is the census's own shape: it records what
upstream PUBLISHES and carries no `held` list (`font-store.ts:201`), so a census written after a
short landing claims nothing false — `familyIsComplete` reads heldness off the face records, finds
the family still short, and goes on offering it. Suppressing the census instead would leave the
family in `unchecked`, whose sentence says this designer has asked upstream nothing, about a family
it has just asked upstream about.

### COMPLETENESS IS THE SAME PREDICATE THAT SELECTED THE FAMILY

A family counts completed when `censusIsComplete(census, kept)` (web/stored) or when every declared
catalogue cut landed (local) — never a third rule of this story's own. `kept` excludes any style
whose store write was refused, so a quota failure reads as a shortfall rather than as a success.

### THE `.sr-only` CHANGE RETIRED AN EXISTING ASSERTION

`App.test.tsx`'s *"hides the offline live region from the painted Preview bar…"* asserted that
Design's span carried **no class at all** — by name, as the half nothing else would notice. The
owner's decision removes that fence, so the test was rewritten rather than deleted: it is now
*"…from the painted bar in both modes…"* and asserts the every-affordance-survives half in BOTH
modes, which is the part that must never quietly become a removal. That rename is the single GONE
entry in the name-set diff.

### RESIDUALS

- **No browser run.** The 136.00 px / +210.00 px / ~424 px figures are the owner's measurement,
  carried into the code comment and into `document-face-completion.test.ts`'s 70-character budget
  assertion; nothing here re-measured them at 1024×768. Playwright was not run (chromium 1217 was
  not exercised). The e2e suites that read `offline-status` use Playwright `toHaveText`, which reads
  `textContent` and does not require visibility, so they are unaffected by the class — checked by
  reading, not by running.
- **`kept.add` is only exercised through a forced quota refusal.** The test installs a `put` that
  refuses on the two FACE stores and lets the census through; that is a `fake-indexeddb` prototype
  patch, not a real quota.


## Spec Change Log

- 2026-09-20 — Re-planned after stories 1, 6, 2 and 3 landed. OQ-2 and OQ-3 settled from
  shipped code (see Design Notes); OQ-1 ruled by the owner for the status bar and then
  escalated on the measurement it required. Code Map anchors re-derived at `6dd0997`.

## Review Triage Log

**2026-09-20, review round 1 — 16 findings, all applied.** The three that changed behaviour rather
than wording:

- **The selection was reading its render closure.** `installOpenedDocument` decides whether to ask
  after three awaits, and all three inputs are late-resolving: `initialLocalFaceHoldings` stands in
  with EMPTY sets wherever there is a release cache, `storeKeepsFaces` is `useState(true)`, and
  `storedFaces` is empty until its listing lands. Both directions were wrong — over-asking for a
  committed family the cache already held whole, and skipping the question in silence for a stored
  family no source had yet been built for. Fixed with refs, and then fixed AGAIN: an
  effect-synced ref is a render behind by construction and still read `true` at selection time in a
  browser whose store had already refused. The refs are now written **where the value is produced**,
  beside their `set*` calls, which is `setCurrentSnapshot`'s own pattern; the offered join is
  re-computed at call time from `storedFacesRef`/`familyCensusesRef`.
- **`storeKeepsFaces` suppressed the question for all three tiers.** A committed family's sink is the
  RELEASE CACHE and never `fontStore` — `installFamily`'s local arm writes nothing to the store by
  design — so a private window with a worker in charge could complete every catalogue family and was
  never asked. The selection now filters by tier instead: `web`/`stored` rows drop when the store
  cannot keep faces, `local` rows stay. The committed-tier describe deliberately installs no machine
  store and is the proof of it.
- **The local arm trusted the response instead of the cache.** An `ok` response the worker declined
  to keep — a failed hash verification, a quota — reported the family completed while
  `holdings.complete` went on excluding it. It now re-probes each fetched URL with `localFaceIsHeld`,
  so its answer and the holdings' cannot disagree.

Also applied: one holdings sweep per RUN rather than per family; the loop breaks on a document
replacement rather than going on spending the author's network; offline is checked before the first
report and again between families, so a mid-run drop ends in the offline sentence rather than a
generic shortfall; the status line gained `role="status" aria-live="polite"` (this same story made
the bar's only other live region visually hidden) and a `mode === 'design'` fence (Preview's bar
states *no network · nothing left this machine*, which a line about fetching upstream would
contradict); terminal outcomes retire on the house `SETTLED_FILE_STATUS_MS` window while a progress
line stands; `completionShortfall` lost the word `still` and the budget note's wrong "59" became a
scanned-and-asserted 60.

**Mutation-checked.** Every fix above was re-run against the mutant that motivated it — render-closure
values, blanket store suppression, no cache re-probe, `!outcome.ok → true`,
`censusIsComplete → true`, unconditional `kept.add`, uncontrolled-page `→ true`, deleted in-loop
progress, deleted pending-question clear, deleted generation break, deleted mid-run offline check,
ungated retire, no retire, unfenced/non-live span, and a no-op `onKeyDownCapture` — and each one reds
at least one test.


Pass 1, 2026-09-20. Three layers: blind-hunter (13 findings), edge-case-hunter (13), verification-gap (6 + 5 other).

| # | Finding | Verdict | Evidence |
|---|---|---|---|
| 1 | Selection reads `storeKeepsFaces`/`browsableFamilies`/`localFaceHoldings` from a stale render closure, after three awaits | **high** | `initialLocalFaceHoldings` returns EMPTY sets when a release cache exists (`held-local-faces.ts:135`), and `storeKeepsFaces` is `useState(true)`. Both directions bite: unsettled holdings ask for cuts already cached; unsettled `storedFaces` finds no source and skips the prompt silently. verification-gap DEMONSTRATED it — inserting the sibling test's two-microtask flush reds both committed-tier tests. |
| 2 | `storeKeepsFaces` gates BOTH arms, but the local arm's sink is the release cache, not the store | **high** | `App.tsx:3669`. A private window with a controlled worker can complete every catalogue family and is never asked. The spec's frozen boundary names `storeKeepsFaces === false` as the test for "nothing can be kept" — true for web/stored, false for local. Exactly one reading of the principle, so patched to it rather than looped back. |
| 3 | `font-completion-status` carries no `role="status"`/`aria-live` | **high** | Same change hid the bar's only live region. Progress, outcome and the offline sentence are announced to nobody. |
| 4 | Local arm returns "completed" from `response.ok`, never re-probing the cache | **medium** | `App.tsx:3381`. Its own docstring claims the return is "every declared cut held". A worker that declines to cache reports `1 family completed.` while `holdings.complete` still excludes it. |
| 5 | No cancellation: only `report` is generation-gated, the fetch loop runs on | **medium** | `App.tsx:3442`. Keeping what landed is a different claim from continuing to spend an author's network on a closed document. |
| 6 | `refreshHeldLocalFamilies()` inside the per-family loop sweeps all 107 catalogue faces each time | **medium** | Exactly the cost `keepOnThisMachine`'s `refresh = false` comment guards against on the store side; the web arm in the same function avoids it. |
| 7 | Offline sampled once, and reported after a progress line it immediately replaces | **medium** | `App.tsx:3445-3448`. A network that drops mid-run reports a generic shortfall — the merge `COMPLETION_OFFLINE`'s own docstring argues against. |
| 8 | A settled completion line never retires | **medium** | Cleared only by the next completion or a new document, so `…1 family completed.` sits in the chrome for the session. The house rule (`App.tsx:110-126`) retires settled statuses and exempts busy ones. |
| 9 | Completion line is not fenced on `mode`, so it renders in Preview | **medium** | Contradicts `no network · nothing left this machine` on the same bar, and the 424px budget was measured for the design bar; preview additionally carries the 228px assurance. |
| 10 | Documented budget says "longest is 59"; `completionShortfall(0, 31)` is 60 and `(0, 256)` is 62 | **medium** | Measured. The test asserts only `<= 70`, so the stated figure is never checked. At the pathological bar (256 fonts, 4-digit revision) 62 chars clips by 14px. |
| 11 | Dialog keyboard contract (Escape, Tab, shortcut-stopping) untested | **medium** | verification-gap: replacing the whole handler with a no-op left 672 tests green. The sibling `UnsavedChangesDialog` is verified all three ways. |
| 12 | Web/stored arm's failure accounting untested | **medium** | verification-gap: making `!outcome.ok` return `true` and `kept.add` unconditional left 672 tests green. A quota-refused write would read as "1 family completed". |
| 13 | Uncontrolled-page branch of the local arm untested | **medium** | verification-gap: flipping its `return false` to `true` left 672 tests green. |
| 14 | Multi-family run untested; the moving numerator is dead code | **medium** | verification-gap: deleting the in-loop progress report left 585 tests green. Every App fixture names exactly one family. |
| 15 | Pending (unanswered) question surviving a document replacement untested | **medium** | verification-gap: removing `setCompletionRequest(undefined)` from `installDocumentIdentity` left 672 tests green. |
| 16 | Committed-tier describe never calls `withMachineStore()` | **medium** | Its coverage is contingent on the store's answer not having arrived — see #1. |
| 17 | Design mode permanently loses its only visible offline indicator | **medium** | Real: `Offline cache unavailable` now has no visible surface. **Rejected as a patch** — the owner ruled unconditional `.sr-only` on 2026-09-20 knowing it changes a shipped surface. Escalated to the coordinator rather than silently re-decided. |
| 18 | `source.faces` empty for a local family → reported complete having fetched nothing | **false** | `offeredFamilies` builds a `local` source by grouping `catalogueFaces` by family, so a family with no faces has no source and is never selected. |
| 19 | `void completeDocumentFamilies(request)` has no `.catch` | **low** | Rejected: no demonstrated throw path — `localFaceIsHeld` catches internally (`held-local-faces.ts:209`), the local fetch is wrapped, and the web arm's helpers return strings rather than throwing. Adding a catch would guard a state never shown reachable. |
| 20 | Concurrent runs / duplicate fetch if the author re-picks the family mid-run | **low** | Rejected: the store is content-addressed and the census write is last-wins, so the worst outcome is a duplicated fetch, not a corrupt state. Fix would add an interlock for an undemonstrated state. |
| 21 | No Stop control for a many-family run | **low** | Rejected: bounded by the document's own family count, and #5's cancellation covers the case that actually strands work. New public surface otherwise. |
| 22 | Test-fixture hygiene: `documentNaming` is a function in one describe and an object in the other; `waitFor` reopens the store each poll | **low** | Real but cosmetic; no named harm. Rejected. |

**Routing:** no `intent_gap` and no `bad_spec` — #2's root cause is a frozen-block parenthetical with exactly one possible reading of its principle, so it is patched to that principle and reported, not looped back. `review_loop_iteration` stays 0. Rows 1-16 route to **patch**; 17 escalated; 18-22 rejected.



**Patch round, 2026-09-20.** All 16 patch-routed findings applied by the step-03 agent and
verified here against the tree, not the report. Three needed more than the stated fix:

- **#1** — effect-synced refs are a render behind by construction, so the refs are written **where
  the value is produced** (beside `setStoreKeepsFaces`, `setStoredFaces`, `setFamilyCensuses`,
  `setLocalFaceHoldings`), and the offered join is recomputed at call time by `offeredForCompletion()`
  rather than read off the `browsableFamilies` memo. Verified at `App.tsx:1001`, `:1018-1019`,
  `:1125-1126`, `:727`, `:1144`, `:3754`.
- **#2/#16** — the tier filter is `.filter((source) => source.tier === 'local' || storeKeepsFacesRef.current)`
  (`App.tsx:3755`). The committed-tier describe deliberately installs **no** store and waits for both
  probes, so it is now the positive proof that a private window with a controlled worker is still
  asked — the opposite precondition from the one the finding proposed, and the honest one.
- **#4/#6** — the local arm re-probes `localFaceIsHeld` after each fetch (`App.tsx:3430`) and the
  holdings sweep moved to once per run. The `cutEmbedPlan` premise comment was reworded so
  all-or-nothing rests on "the cache answers for every declared cut" rather than on the refresh being
  per-family — otherwise another family triggering the single sweep could launder a partial landing.

**Close-out re-measure, 2026-09-20** (chromium 1217, 1024×768, final tree, worst-case content —
4-digit revision, `256 fonts in template`, `256 of 256 elements bound`, longest offline label):

| state | bar | verdict | spacer |
|---|---|---|---|
| no completion, label **painted** | 1024.00 | **fits**, 12 px headroom | 70.00 px |
| completing, label hidden, longest **reachable** line (58 ch) | 1024.00 | **fits**, 12 px headroom | 10.00 px |
| completing, label hidden, the line the code then **documented** (60 ch) | 1026.00 | clipped by 2 px | 0.00 px |

That third row was **unreachable**, and the owner ruled the bound corrected rather than the
sentence trimmed. `document-face-completion.test.ts` had bound the family count by
`addableFamilyCount` (~1811 families an author could *install from*) when the sentence counts the
families *this document names* — a subset of `documentFamilies`, which the projection caps at
`MAX_ENGINE_FONT_FAMILIES`. The scan now **reads that cap** rather than typing a number, so the
budget tracks the engine's own constant instead of silently becoming a lie if it moves. Proven
load-bearing: widening the scanned population reds the test at 61 characters.

**Confirming re-measure after the derived bound** (same worst-case content):

| state | bar | spacer |
|---|---|---|
| idle, label painted | 1024.00 **fits** | 70.00 px |
| completing, **derived** worst case (58 ch) | 1024.00 **fits** | 10.00 px |
| completing, at the ceiling (59 ch) | 1024.00 **fits** | 4.00 px |

The stated worst case and the reachable worst case are now **the same number, 58**, so the code's
fit claim is true rather than documented-as-false. 59 is the measured ceiling and is the asserted
budget (was 70, which could never fire before the real limit). The ~12 px of headroom is recorded
**beside the bar in `App.tsx`**, where the next item added to it will be priced.

**Matrix audit:** all 13 rows covered by a test that ran and passed. One note for the coordinator —
the frozen row *"No store | `storeKeepsFaces === false` | no prompt"* is now over-broad for the same
reason as the frozen boundary in triage row #2: after the tier filter a no-store browser IS asked
about committed families. The row's intent (never ask what a yes cannot act on) is satisfied and both
halves are tested (`Kanit` for the web tier, the committed describe for the local tier). The frozen
text is not mine to edit; reported rather than amended.

## Design Notes

**OQ-3 is settled by the census; both of its options were moot.** Story 1 shipped
`FamilyCensus` (`font-store.ts:201`) recording, per family, what upstream publishes and —
per missing cut — a refusal with `permanence: 'permanent' | 'transient'`.
`censusIsComplete` (`font-store.ts:230`) is exactly "is anything left to fetch": every
published cut held, or refused permanently. Story 3 made the shipped catalogue the local
tier's own census, since `font-catalogue.ts` rows now carry a per-face `style`. So there is
**no probe before consent and no shortfall-guessing**: `familyIsComplete` is the predicate.
One carry-over — a family installed before story 1 has faces but no census row, reads
`unchecked`, and is counted incomplete until a completion establishes its census. That is
the correct reading, not a defect.

**OQ-2 is settled the other way from the hope: the local tier still needs completion.**
`cutEmbedPlan` returns no plan when the family is absent from `completeLocalFamilies`
(`App.tsx:5769`); `cutAbsenceState` then reports `'unfetched'` (`App.tsx:5893`) and the
author reads *"This family has a bold face, but it is not on this machine … Add the family
again to fetch it."* Nothing fetches it: the open-time prefetch covers only faces the
engine actually painted (`document-face-prefetch.ts:83-91`), and an unpressed bold is never
painted. So a catalogue family's cuts are **not** available on demand. The arm is small,
though — `localFaceIsHeld` + a same-origin fetch + `refreshHeldLocalFamilies`, all shipped.

## Verification

**Commands:**
- `cd folio-designer && npm run typecheck` -- expected: clean.
- `cd folio-designer && npm run lint` -- expected: baseline exactly 8
  `only-export-components` warnings, nothing else.
- `cd folio-designer && npm test` -- baseline **89 files / 2108 tests**. Report the
  name-set diff (GONE/NEW), not the total.

**Manual checks (if no CLI):**
- Red-proof the non-mutation AC by issuing a `command` during completion and confirming
  that AC turns red before relying on its green.
