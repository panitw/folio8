---
title: 'New…, Open existing file and the unsaved-changes confirmation'
type: 'feature'
created: '2026-09-15'
status: 'done'
route: 'dispatch'
baseline_commit: '65c78e2580844cba9a56d66ef3bbead68107b5e8'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/specs/spec-startup-templates/SPEC.md'
  - '{project-root}/_bmad-output/planning-artifacts/ux-designs/ux-startup-templates-2026-09-15/mockups/Reopened.dc.html'
  - '{project-root}/_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/DESIGN.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The startup dialog only appears at launch. After that, the document bar's Start blank replaces the document without asking, examples cannot be reached again, and a returning author has to dismiss the dialog before using Open.

**Approach:** Start blank becomes New…, which reopens the dialog (CAP-3). The dialog gains Open existing file… (CAP-8). New… on a document with unsaved changes first asks, naming the document; only after Discard does the dialog open, and nothing is replaced until a choice is made there (CAP-6).

## Boundaries & Constraints

**Always:**
- **New… button.** The document bar's fourth glyph button keeps its place, its `blank` glyph and its disabled conditions. It is renamed: `aria-label` and `data-tip` become "New…". It opens the dialog with Blank selected and focused.
- **Reopened dialog.**
  - Cancel and Escape close it and leave the document, its selection and its undo history untouched.
  - Blank loads the starter exactly as Start blank did, then closes.
  - An example loads as in story 3.
- **Launch dialog.** Unchanged: Blank, Cancel and Escape close it with no engine request.
- **Open existing file…** sits at the left of the footer with the `open` glyph, per `Main.dc.html`, and only when `fileAccess` exists. It uses the same picker and install path as the document bar's Open.
  - Cancelling the picker keeps the dialog open with no message.
  - A successful open closes the dialog and leaves the Designer in its current mode.
  - A failure shows in the footer as `role="alert"`.
- **Warning before the dialog** (owner renegotiation, replaces the in-footer confirmation). When New… is pressed on a document with unsaved changes, a small confirmation opens first, before the startup dialog:
  - It uses `DeletePageDialog`'s shape and `.page-dialog*` styling (`role="dialog"`, `aria-modal`): heading "Discard unsaved changes?", description "{document title} has unsaved changes." with the title in mono after an amber dot.
  - Buttons: "Keep editing" (focused first) and "Discard". Tab stays inside, and Escape means Keep editing.
  - Keep editing closes it: no dialog, nothing requested from the engine, document and undo intact.
  - Discard closes it and opens the startup dialog. Nothing is replaced yet; Cancel or Escape there still leaves the document untouched.
  - With no unsaved changes, New… opens the startup dialog directly.
  - Inside the startup dialog no choice asks again (Blank, an example, Open existing file…): the author has already agreed to discard. The startup dialog has no confirmation footer.
- **Unsaved changes means real edits** (owner decision): the document's revision differs from the one it had when it was last started, opened, opened as an example, or saved. An untouched starter, a just-opened example and a just-opened file (canonical or not) are replaced without asking. The document bar's "Unsaved local changes" label is unchanged.
- **Styling.** Tokens only, and the amber dot uses the existing data token. The focus trap, `aria-disabled` busy state and shortcut gating carry over from story 3.

**Never:**
- No Save-first action in the confirmation.
- No warning on the document bar's own Open, and no `beforeunload` prompt: CAP-6 covers dialog choices only.
- No `window.confirm`, no confirmation stacked over the startup dialog, no new token, no dialog-state persistence.
- No Save sample data (story 5).

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| New…, no changes | No unsaved changes, New… | Startup dialog opens directly, Blank selected; no warning | N/A |
| New…, changes | Unsaved changes, New… | Warning names the document; Keep editing focused; startup dialog not open; nothing requested | N/A |
| Keep editing | Warning showing, Keep editing or Escape | Warning closes, no startup dialog, same revision, Undo still enabled | N/A |
| Discard | Warning showing, Discard | Startup dialog opens; document still unchanged | N/A |
| Cancel after Discard | Discard, then Cancel or Escape in the dialog | Dialog closes; same revision; Undo still enabled | N/A |
| Example after Discard | Discard, then Invoice | Invoice opens in Preview with no further question | Load failure shows alert, dialog stays open |
| Blank after Discard | Discard, then Start blank | Starter loaded, title "Untitled template", dialog closed | Same as Start blank's failure |
| Open file | Open existing file…, pick valid `.folio` (at launch or after Discard) | Dialog closes, file opened as by the bar's Open; no further question | Invalid file: alert in footer, dialog open |
| Picker cancelled | Open existing file…, cancel picker | Dialog open, no alert, document untouched | N/A |

</frozen-after-approval>

## Code Map

- `folio-designer/src/App.tsx:2960-2976` (`startBlank`): its body becomes the Blank path of a reopened dialog. The document-bar button at `:3491` changes to New… (opens the dialog).
- `folio-designer/src/App.tsx:2861-2896` (`open`, `installOpenedDocument`): split the picker from the install so the dialog shares the path. Cancel is detected with `isFileAccessCancelled` (`src/file/file-access.ts:65`).
- `folio-designer/src/App.tsx:2904-2934` (`chooseStartup`), `:372-377` (dialog state, `startupCards`): add the dialog origin (launch or New…) and a pending confirmation. `startupCards` must still hold Blank when `examples` is undefined, so New… works in unit tests.
- `folio-designer/src/App.tsx:366,3256` (`savedRevision`, `dirty`): keep `dirty` for the bar's label. Add a separate baseline revision, set wherever a document is installed (starter at launch, `startBlank`, `open`, example load) or saved. The warning fires only when `snapshot.revision` differs from that baseline.
- `folio-designer/src/StartupDialog.tsx`: add `onOpenFile?` and `onCancel`. Cancel and Escape stop meaning Blank; App decides. (After the owner renegotiation: no confirmation prop.)
- `folio-designer/src/App.tsx` `DeletePageDialog` (~:5772): the shape copied by `UnsavedChangesDialog`, the warning New… shows before the startup dialog.
- `folio-designer/src/App.css` `.startup-*`, `.unsaved-warning-*`: Open existing file, the footer rule, and the warning's amber dot and mono title.
- **Tests clicking the document bar's "Start blank"** move to New… then Start blank, plus Discard when a confirmation appears, through one shared unit-test helper:
  - `src/App.test.tsx`: 2942, 6091, 6172, 8583, 9015, 9554, 10733; names at 1169 and 1192.
  - `src/App.font-store.test.tsx`: 314, 373, 851, 919.
  - `src/DataPanel.test.tsx`: 263, 297, 864.
  - `src/table-column-binding.test.tsx`: 151, 167.
  - `e2e/pages.spec.ts`: 30, 95, 164.
  - `e2e/application-shell.spec.ts`: 25.
- `folio-designer/src/control-vocabulary-contract.test.tsx:585,1014`: rename "Start blank" to "New…"; leave the planted-violation fixtures at 810-828 alone.
- **Undo history** lives in the engine (`folio-go/wasm/engine.go` `load` clears it). Keep editing must make no engine request.

## Tasks & Acceptance

**Execution:**
- [x] `folio-designer/src/StartupDialog.tsx`: Open existing file…, separate Cancel, confirmation footer and its focus and Escape behaviour. It stays presentational.
- [x] `folio-designer/src/App.css`: styles for the new footer states, tokens only.
- [x] `folio-designer/src/App.tsx`: New… button, dialog origin, the Blank path for a reopened dialog, open-from-dialog, the unsaved predicate and confirmation state, all wired to the shared install helpers.
- [x] `folio-designer/src/App.test.tsx`: cover every I/O matrix row, including undo still enabled after Keep editing and after Cancel. Add the shared New…-then-Blank helper and migrate the unit tests listed in the Code Map.
- [x] `folio-designer/src/control-vocabulary-contract.test.tsx`: rename the button.
- [x] `folio-designer/e2e/startup-dialog.spec.ts`: add three flows:
  - edit, then New…, Invoice, Keep editing (same revision), then Discard lands in Preview;
  - Open existing file… with a fixture `.folio` closes the dialog;
  - after migrating `pages.spec.ts` and `application-shell.spec.ts`, confirm they still pass.

**Rework after owner renegotiation (warning before the dialog):**
- [x] `folio-designer/src/StartupDialog.tsx`, `src/App.css`: remove the confirmation footer, the `confirmation` prop, the `.startup-unsaved*` rules and the confirmation-only focus handling. Keep Open existing file…, `onCancel` and the Tab-trap fix.
- [x] `folio-designer/src/App.tsx`: New… checks for real edits and shows an unsaved-changes warning in `DeletePageDialog`'s shape. Discard opens the startup dialog, and Keep editing/Escape closes the warning. Remove `startupPending` and the in-dialog asking from `chooseStartup`/`requestStartupFile`. App keyboard shortcuts and canvas keys stay gated while the warning is open.
- [x] `folio-designer/src/test/new-document.ts`, `e2e/app.ts`: `startBlankFromNew` presses Discard on the warning if it appears, then Start blank.
- [x] `folio-designer/src/App.test.tsx`: replace the confirmation tests with tests for every row of the revised matrix.
- [x] `folio-designer/e2e/startup-dialog.spec.ts`: rework the edited-document flow. Edit, New…, and the warning shows before any startup dialog. Keep editing leaves the same revision and no dialog. New… again, Discard, and the dialog opens; Invoice lands in Preview.

**Acceptance Criteria:**
- Given the warning is showing, when Tab is pressed repeatedly, then focus cycles only through Keep editing and Discard, and no startup dialog is in the page.
- Given `npx playwright test` against the production build, when run, then every spec passes.

## Implementation Notes

- **Unsaved predicate:** `baselineRevision` in App.tsx starts at `initialSnapshot.revision`. It is set in `installOpenedDocument` (covers Open, Open existing file… and examples), in `startBlank`, and on a save that wrote the current revision. The warning fires only when `snapshotRef.current.revision !== baselineRevision`. `savedRevision`/`dirty` (the bar label) are unchanged.
- **Dialog state:** `startupOrigin` ('launch' | 'new') and `startupPending` (blank | example | file). At launch, Blank, Cancel and Escape only close the dialog. After New…, Blank calls `startBlank` (which now returns a boolean). On failure the dialog stays open with "Could not start a blank local template" in the footer.
- **Open existing file…:** `open` was split. After the picker, both the document bar and the dialog call `installPickedFile(opened)`. The dialog path calls `fileAccess.open()` before any await, so the click's user activation still holds. The preview/sample invalidation runs only after a file is picked, so a cancelled picker changes nothing. A failure shows as `role="alert"` in the footer.
- **StartupDialog:** new `onCancel`, `onOpenFile?` and `confirmation` (`{document, question, discard, onKeep, onDiscard}`).
  - While the confirmation shows: Escape means Keep editing, Tab cycles only through the footer's two buttons, and cards are `aria-disabled` and ignore clicks.
  - Focus moves to Keep editing. When the confirmation goes away, focus returns to the last element focused in the dialog, or to the primary action if that element was replaced.
  - Keep editing has `aria-describedby` pointing at the message.
  - The Tab trap now also pulls focus back in when focus is on the dialog section itself.
- **Mockup reading:** `Reopened.dc.html` shows the footer without Open existing file…, so the confirmation replaces the whole footer. Open existing file… is not rendered (rather than `aria-disabled`) while the confirmation shows.
- **Test helpers:**
  - `src/test/new-document.ts` `startBlankFromNew()`: New…, then Start blank, then Discard if asked. Used by App, font-store, DataPanel and table-column-binding tests.
  - `e2e/app.ts` `startBlankFromNew(page)`: used by `pages.spec.ts`. `application-shell.spec.ts` now checks for "New…".
- **Verification:**
  - `npm run build`: 0, including `verify:offline`.
  - `npx vitest run`: 1846 in 86 files, all passing after fixing one missed migration in table-column-binding.
  - `tsc -b`: 0. `test:e2e:compile`: 0. `lint`: 0 errors, only existing warnings.
  - `npx playwright test`: 135 passed (6.5 min), including the two new startup-dialog flows.
- **Not done:** the manual visual comparison with `Reopened.dc.html` / `Main.dc.html` in the production build.
- **Rework after owner renegotiation (warning before the dialog):**
  - **StartupDialog:** the `confirmation` prop, the confirmation footer and its focus-return and Escape handling are gone. Open existing file…, `onCancel` and the Tab-trap pull-in stay. The `.startup-unsaved*` rules are removed from App.css.
  - **Warning:** new `UnsavedChangesDialog` in App.tsx, a copy of `DeletePageDialog`'s shape using `.page-dialog-backdrop` / `.page-dialog` / `.page-dialog-actions`.
    - Heading "Discard unsaved changes?". Description: amber dot, then the title in mono, then "has unsaved changes." (`.unsaved-warning-dot`, `.unsaved-warning-document`, existing tokens only).
    - Buttons in order Keep editing (focused first), then Discard (`.page-dialog-confirm`). Tab toggles between them, Escape means Keep editing, and a backdrop press keeps focus on Keep editing.
  - **App wiring:** `startupPending` is replaced by `unsavedWarningOpen`.
    - New… (`openNewDialog`) checks `unsavedEdits()`. With real edits it opens the warning; without, it opens the dialog directly.
    - Keep editing only closes the warning. Discard closes it and opens the dialog (origin 'new', Blank selected) without replacing anything.
    - `chooseStartup` and `requestStartupFile` no longer ask. App shortcuts (including the Cmd/Ctrl+S suppression) and canvas keys are gated while the warning is open. `baselineRevision`, the dialog origin, `installPickedFile`, the picker inside the click's activation and the `startBlank` boolean are unchanged.
  - **Helpers:** `startBlankFromNew` (unit and e2e) presses Discard on the warning when it appears, then Start blank.
  - **Tests:** the story-4 unit block was replaced with one test per row of the revised matrix, plus the button rename and glyph, Blank without examples, just-opened example and just-saved document (no warning), Tab confined to the warning with no dialog in the page, and shortcuts gated while the warning is open. The e2e edited-document flow now covers: warning before any dialog, Tab cycling, Keep editing with same revision and Undo, then New…, Discard, dialog, Invoice in Preview.
  - **Verification (foreground):**
    - `npm run build`: 0, including `verify:offline`.
    - `npx vitest run`: 1847 passed in 86 files.
    - `tsc -b`: 0. `test:e2e:compile`: 0. `lint`: 0 errors, only existing warnings.
    - `npx playwright test`: 135 passed (6.6 min).
  - **Still not done:** manual visual check of the warning in the production build.

- **Review patches (triage rows 1–5):**
  - `UnsavedChangesDialog` section is `tabIndex={-1}`.
  - The example-load comment was moved above `openExample`.
  - New tests: Blank from New… resets the baseline; the section-focused Tab trap; the open-file e2e now checks the status line.
- **Post-patch verification:**
  - `npm run build`: 0, including `verify:offline`.
  - `npx vitest run`: 1850 passed in 86 files.
  - `typecheck`: 0. `test:e2e:compile`: 0. `lint`: 0 errors.
  - `npx playwright test`: 135 passed (6.6 min).
- **Manual check** (production build, 1440×900): the warning renders in the `.page-dialog` shape with an amber dot, mono "Untitled template", and Keep editing and Discard. The launch dialog footer shows Open existing file… at the left and Cancel and Start blank at the right.

## Spec Change Log

- **2026-09-15 — owner renegotiation during implementation.** Triggered by the owner, not a review finding. The unsaved-changes warning now appears when New… is pressed, before the startup dialog opens, instead of inside the dialog's footer when a choice is confirmed. Amended in the frozen block: Approach, the Always bullet (warning before the dialog, which replaces the confirmation-footer and warn-before-picker bullets), Never (no confirmation stacked over the dialog), and the I/O matrix. Known-bad state avoided: an author with edits picked a template first and was only then told their work would be lost. KEEP: `baselineRevision` as the real-edits predicate, dialog origin (launch vs New…), `installPickedFile` shared by Open and Open existing file…, the picker called inside the click's activation, the `startBlank` boolean result, Open existing file… in the footer, `onCancel`, the Tab-trap pull-in fix, and the shared `startBlankFromNew` helpers.

## Review Triage Log

| # | Layer | Finding | Verdict | Evidence | Route |
|---|---|---|---|---|---|
| 1 | edge | Clicking the warning's heading or text drops focus to body; Escape and Tab stop working | medium | `UnsavedChangesDialog` section has no `tabIndex` and `holdFocus` only catches presses outside `.page-dialog`, so a press on text inside it moves focus to body, outside the `onKeyDownCapture` handler | patch |
| 2 | verification-gap | No test that Blank from New… resets the baseline, so a second New… skips the warning | medium | Pre-verified: the Blank-after-Discard test stops after load; the helpers press Discard only if the warning appears | patch |
| 3 | verification-gap | StartupDialog Tab from the focused section (`index === -1`) untested | low | Pre-verified: existing trap tests press Tab only from buttons; a direct test addition | patch |
| 4 | blind | Example-load doc comment now sits above `unsavedEdits`/`chooseStartup`, not `openExample` | low | Diff shows the comment above the new helpers, with `openExample` about 60 lines down; direct correction | patch |
| 5 | blind | e2e Open existing file… passes without proving the fixture loaded | low | Asserts `.document-name` and `Canvas region`, which is always visible; one status assertion fixes it | patch |
| 6 | blind, edge | Undo back to the baseline content still warns | low | Real: engine restore bumps revision. But the frozen spec defines unsaved as a revision different from the baseline, so the code matches it, and a content comparison adds new state | rejected |
| 7 | blind | Command in flight (e.g. blur-committed section-break Y) not counted when New… is pressed | low | Commits have no shared in-flight tracker, so the check can run before the revision moves. The window is milliseconds on the one blur-committing inspector field (TableEditor is modal); a fix needs a new counter | rejected |
| 8 | blind, edge | A failed Blank from the dialog shows its error in both the footer and the bar | low | `startBlank` calls `announceFailure` as before; a starter load failure is rare, and the fix adds a parameter | rejected |
| 9 | blind, edge | `startBlank` returning false for busy shows a false failure | false | New… is disabled while `fileBusy`, the startup dialog is modal over the bar, and `startupBusy` guards re-entry, so `startBlankFromStartup` never runs while `fileBusy` | rejected |
| 10 | blind, edge, verification-gap | Open existing file… is not locked while the picker is open | low | `showOpenFilePicker` and `<input type=file>` open OS-modal pickers, so the page takes no clicks while one is up; the bar's Open relies on the same | rejected |
| 11 | blind, edge | A failed open after Discard has already cleared preview parameters and the render | low | Same order and effect as the document bar's Open (`open` clears before its picker); rare invalid file | rejected |
| 12 | blind | Focus is not returned to New… when the warning or dialog closes | low | Matches `DeletePageDialog`, `FontBrowser` and `TableEditor`; same call as story 3 triage row 8 | rejected |
| 13 | blind | Mode test only covers opening from Design | low | `installPickedFile`'s Preview branch is the bar's Open code, unchanged and covered there | rejected |
| 14 | blind | No tests for non-canonical open, Save As or a superseded save against the warning | low | `installOpenedDocument` sets the baseline unconditionally and `save` only on the written current revision; trivial paths | rejected |
| 15 | blind | Unit and e2e `startBlankFromNew` helpers wait differently | low | Unit callers await their own settle conditions; 1847 pass | rejected |
| 16 | edge | `DeletePageDialog` has the same focus escape on a click inside its text | medium | Pre-existing (same `holdFocus` shape, no `tabIndex`) | defer |

## Design Notes

**One dialog, two origins.** At launch the untouched starter already is Blank, so closing is enough. After New…, Blank must actually replace whatever is open.

**Warning before the dialog.** Asking at New… means the author decides about their edits before browsing templates. The dialog then stays one unbroken choice, and Cancel still loses nothing, since Discard only agrees to replacement and does not perform it.

## Verification

**Commands:**
- `cd folio-designer && npm run build`: exits 0, including `verify:offline`.
- `cd folio-designer && npx vitest run`: passes, including `canvas-authority-contract` (e2e files must not read `scroll*`/`client*`/`offset*` sizes).
- `cd folio-designer && npm run typecheck && npm run test:e2e:compile && npm run lint`: passes.
- `cd folio-designer && npx playwright test`: passes.

**Manual checks:**
- In the production build, check the unsaved-changes warning against `DeletePageDialog`'s look, and the footer with Open existing file… against `Main.dc.html`.
