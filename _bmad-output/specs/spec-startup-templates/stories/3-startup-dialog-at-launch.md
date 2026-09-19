---
title: 'Startup dialog at launch'
type: 'feature'
created: '2026-09-15'
status: 'done'
route: 'dispatch'
baseline_commit: 'fb6a1f7150f22c01068c7c46e3418f7362a15e7c'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/specs/spec-startup-templates/SPEC.md'
  - '{project-root}/_bmad-output/planning-artifacts/ux-designs/ux-startup-templates-2026-09-15/mockups/Main.dc.html'
  - '{project-root}/_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/DESIGN.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The four examples ship in the release (stories 1–2) but nothing opens them; a new author still lands on an empty page with no way to see a finished, previewable document.

**Approach:** On every launch, once the engine is ready, show a "New template" dialog with Blank and the four examples as thumbnail cards (SPEC CAP-1). Blank or Escape keeps today's starter; an example fetches its template and sample, loads both exactly as Open and sample-load do, titles the document with the example's name, leaves it unsaved with no file target, and switches to Preview (CAP-2, CAP-5).

## Boundaries & Constraints

**Always:**
- Visual and copy follow `Main.dc.html` and DESIGN.md: 940 px sheet over `--tint-scrim` with `--shadow-sheet`; 46 px header "New template" + hint; cards in the order Blank, Invoice, Bank Statement, Legal Contract, Electricity Bill, each with its engine-rendered thumbnail on `--color-page-shell`, name, one-line description, and the sample file name (amber, mono) or "no sample data"; selected card cyan; footer states what the selection does and holds the primary action ("Start blank" / "Open example") beside a Cancel button. Narrower than five cards, the grid wraps and thumbnails scale down inside their cards. Tokens only — no colour literals, square corners, no new shadow or token.
- Names and descriptions: Blank "Empty A4 page"; Invoice "Line items, totals, payment QR"; Bank Statement "Paginated transactions"; Legal Contract "Clauses, signature block"; Electricity Bill "Usage, charges, barcode".
- Blank is selected when the dialog opens and holds initial focus. Cards are `aria-pressed` buttons in a labelled `role="group"`; click selects, Enter/double-click or the primary action confirms. The dialog is `role="dialog"` `aria-modal` with the existing inline focus-trap shape, and App keyboard shortcuts do not fire while it is open.
- Blank, Cancel or Escape closes the dialog without an engine request: the starter the engine already holds stays at revision 1.
- An example load reuses Open's document path and sample-load's accept path (factored into shared helpers, not duplicated), sets title to the example name, `target` and `savedRevision` undefined, then enters Preview once; the render uses the sample, so the "No sample data" notice is absent. Files are fetched from `exampleAssets` URLs with `credentials: 'omit'`.
- A failed fetch or load keeps the dialog open with the failure in the footer (`role="alert"`); Blank still works.
- The dialog appears only when App receives the examples from `main.tsx`; App mounted without them (unit tests) behaves as today.

**Never:**
- No "Open existing file…", no New… in the document bar, no unsaved-changes confirmation (story 4); no Save sample data (story 5).
- No localStorage/IndexedDB for dialog state; no build-time env or query flag that skips the dialog.
- No change to `example-assets.ts` generation or to example files.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Launch | Engine ready, examples passed | Dialog open, Blank selected and focused, five cards with thumbnails | N/A |
| Blank | Blank confirmed, Cancel, or Escape | Dialog closes; canvas shows the starter at revision 1; no engine request | N/A |
| Open example | Bank Statement confirmed | Title "Bank Statement"; unsaved, no target; Preview renders with its sample; no "No sample data" notice; sample tree available to binding | N/A |
| Fetch fails | Example URL returns non-OK | Dialog stays open, nothing loaded | Footer alert names the example; Blank still closes |
| No examples prop | App mounted without examples | No dialog | N/A |

</frozen-after-approval>

## Code Map

- `folio-designer/src/main.tsx:9-13,29-38` -- replace the side-effect import with `import { exampleAssets }`; pass it to App once the engine is ready.
- `folio-designer/src/App.tsx:259,327` -- `AppProps`; add the examples prop. App remounts on engine ready (`key`), so launch-only state initialises once.
- `folio-designer/src/App.tsx:2813-2837` (`open`) -- factor the post-picker body into a helper taking bytes, name, target; `open` and the example load both call it.
- `folio-designer/src/App.tsx:2795-2812` (`loadSample`) -- factor the accept tail (`acceptSampleData` → `sampleDataRef`/`setSampleData` → preview invalidation) into a helper taking name and bytes.
- `folio-designer/src/App.tsx:1195-1200` (`enterPreview`) -- sets `modeRef` synchronously; call after document and sample install.
- `folio-designer/src/App.tsx:3034-3053,1736` -- keyboard shortcuts and the `fileBusy` guard; gate while the dialog is open, set `fileBusy` during an example load.
- `folio-designer/src/App.tsx:3545-3546` -- where TableEditor and FontBrowser render inline; render the dialog alongside.
- `folio-designer/src/FontBrowser.tsx:109,127-145,276` -- dialog markup, initial focus and inline focus trap to copy.
- `folio-designer/src/App.css:1234-1331` -- `.font-browser-*` sheet, header, grid/card, footer, confirm and active-state rules to mirror under new `.startup-*` classes.
- `folio-designer/src/design-contract.test.ts:52-101` -- no colour literals, `var(--radius…)` only, reuse `--shadow-sheet`, focus outline rule.
- `folio-designer/src/App.test.tsx:210,2941,8548,3921` -- fake engine, blank start, sample load and hand-rolled `fetch` stub patterns.
- `folio-designer/e2e/*.spec.ts` -- 37 specs, 69 `page.goto(` calls land on the canvas today; a production build serves the dialog in e2e (`playwright.config.ts` webServer).

## Tasks & Acceptance

**Execution:**
- [x] `folio-designer/src/startup-examples.ts` -- id → name and description for Blank and the four examples, plus the sample file name -- dialog copy in one place.
- [x] `folio-designer/src/StartupDialog.tsx` -- the dialog per Boundaries -- presentational, receives cards, selection, busy and error, and callbacks.
- [x] `folio-designer/src/App.css` -- `.startup-*` rules mirroring the font-browser sheet with the mockup's sizes -- tokens only.
- [x] `folio-designer/src/App.tsx` -- examples prop, open-at-launch state, document and sample install helpers shared with `open`/`loadSample`, example load, shortcut gating, render -- wiring.
- [x] `folio-designer/src/main.tsx` -- import and pass `exampleAssets` -- enables the dialog in the product only.
- [x] `folio-designer/src/App.test.tsx` (or `StartupDialog.test.tsx`) -- every I/O matrix row, the metadata covering every `exampleAssets` id, focus trap and shortcut gating -- unit coverage.
- [x] `folio-designer/e2e/app.ts` (new) and every `e2e/*.spec.ts` that navigates to the app -- a shared `openWorkspace(page)` that navigates and dismisses the dialog with Escape; replace direct `page.goto('/')` landings -- existing specs keep their meaning.
- [x] `folio-designer/e2e/startup-dialog.spec.ts` -- launch shows the dialog; Escape lands on the canvas; opening Invoice lands in Preview with its sample loaded -- end-to-end proof.

**Acceptance Criteria:**
- Given a production build, when the app launches offline after first load, then the dialog and all five thumbnails appear and an example opens in Preview.
- Given the dialog is open, when Tab is pressed repeatedly, then focus cycles within the dialog, and Cmd/Ctrl shortcuts do nothing.
- Given `npx playwright test`, when run, then every pre-existing spec passes through the shared helper.

## Implementation Notes

- Shared helpers: `installOpenedDocument(bytes, name, target)` (Open and example load) and `acceptSample(name, bytes, stillCurrent)` (Load sample JSON and example load). `chooseStartup` fetches both files before any engine request, so a failed fetch leaves the starter untouched.
- Dialog copy lives in `src/startup-examples.ts`; `startup-examples.test.ts` holds it to `exampleAssets` (same ids, same order, sample names match the fingerprinted files).
- Busy is `aria-disabled` rather than `disabled`, so focus stays inside the trap while an example opens.
- e2e: `e2e/app.ts` `openWorkspace(page)` navigates, waits for the dialog and Invoice focus, presses Escape; 37 specs switched to it. `engine-worker.spec.ts` and `offline-update.spec.ts` reload after landing, and the dialog reappears on reload.
- Manual check against the production build (1440×900): dialog matches `Main.dc.html` — header and hint, START/EXAMPLES labels, five cards with engine thumbnails, Invoice selected cyan with amber sample name (long names ellipsised), footer outcome and "Open example". Opening Invoice lands in Preview titled "Invoice", unsaved, rendered from its sample with 0 warnings and 0 errors.
- e2e: `npx playwright test` against the production build — 130 passed (6.5 min), 0 failed, including the 37 migrated specs, the two that reload after landing, and `startup-dialog.spec.ts`.
- Review patches (triage rows 1–6): `chooseStartup` parses the sample with `acceptSampleData` right after both fetches, before any engine request or state change, then installs it with the new `installAcceptedSample` (Load sample JSON still goes through `acceptSample`); `fetchExampleFile` aborts at `ENGINE_FILE_STEP_TIMEOUT_MS` with "its bundled file did not arrive in time"; the dialog section is `tabIndex={-1}` so a click inside keeps focus in the trap; Cmd/Ctrl+S is `preventDefault`ed while the dialog is open; both whitespace slips restored. New unit tests: rejected sample, engine `load` rejection, stalled fetch (fake timers), title click then Escape; the shortcut test asserts Save is `defaultPrevented`.
- Post-patch verification: `npm run build` 0 incl. `verify:offline`; `npx vitest run` 1823/1823 in 86 files; `typecheck` 0; `lint` 0; `npx playwright test` 130 passed.
- Cosmetic (fixed in review): two whitespace slips — `examples={…}offlineState` in `main.tsx` and `paletteItems … =[[` in `App.tsx`.

## Spec Change Log

- **2026-09-15 — owner renegotiation after trying the dialog.** Triggered by the owner, not a review finding. Amended inside the frozen block at the owner's request: a Cancel button beside the primary action (same outcome as Escape); Blank, not Invoice, selected and focused on open; the card grid wraps and thumbnails scale down at narrow widths. Cancel and the primary action are grouped (`.startup-actions`) so a wrapping footer keeps them together, right-aligned. Checked in the production build at 1440, 720 and 480 px: no sideways scroll, no thumbnail outside its card, Blank focused, both buttons on one row. Known-bad state avoided: at narrow widths five fixed 132 px thumbnails in five squeezed columns overflowed their cards and overlapped. KEEP: shared install helpers, sample parsed before any engine request, fetch deadline, focusable dialog section, Save shortcut suppressed while open.

## Review Triage Log

| # | Layer | Finding | Verdict | Evidence | Route |
|---|---|---|---|---|---|
| 1 | blind, edge, verification-gap | Sample rejected after `installOpenedDocument` leaves a half-opened example; Escape then closes onto it | medium | `acceptSample` runs after the document, title and identity are already replaced; the dialog stays open and Blank closes without reverting — contradicts "Blank still works" | patch |
| 2 | verification-gap | No test for an engine `load` rejection after both fetches succeed | medium | Pre-verified: the only failure test is a fetch 404, which never reaches the install block | patch |
| 3 | blind, edge | Example fetch has no deadline; a stalled fetch leaves the dialog busy with Escape and every action ignored | medium | `fetchExampleFile` has no signal, unlike `engineFileStep`; `startupBusy` never clears | patch |
| 4 | blind, edge | A click on a non-focusable part of the dialog drops focus to `body`; Escape stops working and Tab reaches the workspace | medium | Escape/Tab handling is `onKeyDownCapture` on the section, which only fires while focus is inside it | patch |
| 5 | edge | Cmd/Ctrl+S opens the browser's Save Page dialog while the startup dialog is open | low | The shortcut handler returns on `startupOpen` before its own `preventDefault`; direct correction | patch |
| 6 | blind | Whitespace slips in `main.tsx` (`}offlineState`) and `App.tsx` (`=[[`) | low | Direct correction | patch |
| 7 | blind | Specs that reload after `openWorkspace` run with the dialog open again | low | All 130 e2e tests pass; those specs only assert text after reload and click nothing the dialog covers | rejected |
| 8 | blind | Focus is not placed after the dialog closes | low | Matches FontBrowser and TableEditor, neither of which restores focus; not a regression | rejected |
| 9 | blind | Card description and sample name read without a separator | low | `aria-describedby` joins referenced text with a space; readable as is | rejected |
| 10 | blind | Five-column grid overflows at narrow widths | false | `.app-shell` has `min-width: 1024px`; the dialog is `min(940px, 100vw - 44px)` inside the supported width | rejected |
| 11 | blind | Dialog "Start blank" and document-bar "Start blank" share a name | low | The bar button is renamed to New… in story 4; no current spec is ambiguous (130 pass) | rejected |
| 12 | blind | "Open example" silently does nothing while `fileBusy` | false | Nothing sets `fileBusy` before the dialog closes at launch; the branch is unreachable | rejected |
| 13 | blind | Selected id can disagree with the drawn card if Invoice is missing | false | `startup-examples.test.ts` fails the build if the default id is not bundled | rejected |
| 14 | blind | Missing unit tests for busy state, network rejection, examples without engine | low | Load failure and sample rejection are covered by rows 1–2; the rest are unlikely in everyday use | rejected |
| 15 | blind | A network error surfaces the browser's raw "Failed to fetch" text | low | Offline launches are served from the verified cache; wording only | rejected |
| 16 | blind | Footer swaps a status region for an alert region | low | Same pattern as FontBrowser's footer (`FontBrowser.tsx:343,354`) | rejected |
| 17 | blind | e2e opens only Invoice and Bank Statement | low | All four render through the engine in the Go examples test and the build gate | rejected |
| 18 | edge, verification-gap | `serialize` failing after a successful `load` leaves engine and UI diverged | medium | Pre-existing in Open's path, now shared by the example load | defer |
| 19 | blind, edge | Tab trap relies on focus already being inside the dialog | medium | Same inline trap shape in FontBrowser and TableEditor; fixed here for the startup dialog (row 4) | defer (existing dialogs) |

## Design Notes

EXPERIENCE.md lists "Not a wizard … no first-run tour beyond the load screen's explanation" and "no theme gallery". This dialog is a user-requested exception recorded in SPEC CAP-1: one choice, dismissible with Escape to exactly today's canvas, no steps. EXPERIENCE.md itself is amended outside this build.

Escape keeps the starter rather than calling `startBlank`, so launch state stays at revision 1 and e2e specs only need dismissal, not new expectations.

## Verification

**Commands:**
- `cd folio-designer && npm run build` -- expected: exits 0 including `verify:offline`.
- `cd folio-designer && npx vitest run` -- expected: pass.
- `cd folio-designer && npm run typecheck && npm run lint` -- expected: pass.
- `cd folio-designer && npx playwright test` -- expected: pass.

**Manual checks (if no CLI):**
- Launch the built app; compare the dialog with `Main.dc.html`; open each example and confirm Preview shows its sample.
