---
title: 'Save sample data'
type: 'feature'
created: '2026-09-16'
status: 'done'
route: 'dispatch'
baseline_commit: '5677008f0c441752feb77af17a53472e11467315'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/specs/spec-startup-templates/SPEC.md'
  - '{project-root}/_bmad-output/planning-artifacts/ux-designs/ux-folio-2026-08-23/DESIGN.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** An example's sample data lives only in memory. Reload the page and the author can rebuild the template from a saved `.folio`, but the sample that made its Preview real is gone, and there is no way to get the file the Designer loaded.

**Approach:** A Save sample data control beside Load/Replace sample JSON writes the loaded sample's exact bytes to a local file through the existing file access, so the saved JSON reopens as sample data and restores the Preview (SPEC CAP-7).

## Boundaries & Constraints

**Always:**
- **Byte for byte.** The file written is `sampleData.bytes` exactly as accepted — no re-serialization, no reformatting, whatever the tree was truncated for display.
- **The control** sits in the sample data panel immediately after Load/Replace sample JSON, labelled "Save sample data". It appears only when a sample is loaded, and is disabled while a file operation is in flight or when there is no local file access — the same conditions the panel's existing button answers to.
- **The save** mirrors `exportPreviewPdf`: `acquireSaveTarget` with `saveAs: true` and a JSON sample format (`application/json`, `.json`), then `writeSave`. The suggested name is the loaded sample's own name. No target is remembered, so every save asks where to put it.
- **Reporting** is the document bar's file status, as PDF export reports: "Saved sample data as {name}" with a picker, "Downloaded sample data {name}" without one. A cancelled picker says nothing. A failure goes through `announceFailure`.
- **The document is untouched:** no engine request, no revision change, no effect on `savedRevision`, the unsaved-changes baseline, the title or the file target.
- **Availability** does not depend on where the sample came from: a sample from Load sample JSON saves exactly as an example's does.
- Visual language is DESIGN.md's, tokens only, reusing the panel's existing button treatment.

**Never:**
- No new sample format written into the `.folio`, and no path to the sample stored in the document.
- No auto-save, no remembered target or directory, no "save both" combined action.
- No change to how samples are loaded, parsed, truncated or bound.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| No sample | Nothing loaded | No Save sample data control | N/A |
| Save with picker | Sample loaded, file access with pickers | Picker suggests the sample's name; bytes written equal the loaded bytes; bar reads "Saved sample data as {name}" | N/A |
| Save by download | Sample loaded, download fallback | Blob is `application/json`, named from the sample; bar reads "Downloaded sample data {name}" | N/A |
| Cancelled | Picker dismissed | Nothing written, no message, nothing disabled | N/A |
| Write fails | `writeSave` throws | Bar alert names the failure | `announceFailure`, `fileBusy` released |
| Busy | A file operation in flight | Control disabled; a click does nothing | N/A |
| Truncated sample | Sample whose tree was truncated for display | The full original bytes are written, not the truncated view | N/A |

</frozen-after-approval>

## Code Map

- `folio-designer/src/sample-data.ts:22` — `SampleData.bytes` holds the accepted bytes (`bytes.slice(0)`); `sampleData.name` is the bounded file name.
- `folio-designer/src/App.tsx:536,623` — `sampleData` state and `sampleDataRef`; `:3151-3181` `exportPreviewPdf` is the pattern to copy (guards, `fileBusy`, `acquireSaveTarget` + `writeSave`, cancel and failure handling, "Saved … as" vs "Downloaded …").
- `folio-designer/src/file/file-access.ts` — `LocalFileFormat`, `folioFileFormat`, `pdfFileFormat`, `localFileName` (strips one known extension). Add the JSON sample format beside them; `file-system-access.ts` `pickerTypeFor` and `input-download.ts` already work from any format.
- `folio-designer/src/DataPanel.tsx:69-70,145-147` — props, the `action` label, the `file-button` markup, and the panel's `role="alert"` line; App wires the panel at `App.tsx:3699`.
- `folio-designer/src/sample-file.ts:7` — `samplePickerType` describes the same JSON type for opening; keep one wording for both.
- Tests to follow: `src/App.test.tsx:1922-1924` (an `acquireSaveTarget` stub), `:8551` and `src/DataPanel.test.tsx:137,295` (sample access stubs), `e2e/pdf-export.spec.ts:36-54,74-114` (download tier and picker tier), `e2e/sample-data.spec.ts:8-27` (loading a sample in the browser).

## Tasks & Acceptance

**Execution:**
- [x] `folio-designer/src/file/file-access.ts` — add the JSON sample format constant beside the others — one description and extension for every save path.
- [x] `folio-designer/src/App.tsx` — a `saveSampleData` following `exportPreviewPdf`, wired to the panel — the save itself.
- [x] `folio-designer/src/DataPanel.tsx` — the Save sample data button and its `onSave` prop — the control.
- [x] `folio-designer/src/App.test.tsx` and `src/DataPanel.test.tsx` — every matrix row, including the bytes written being identical to the loaded bytes — unit coverage.
- [x] `folio-designer/e2e/sample-data.spec.ts` — save the loaded sample through the download tier and compare the downloaded bytes with the file that was loaded — browser proof.

**Acceptance Criteria:**
- Given an example opened from the startup dialog, when its sample is saved and the saved file is loaded again with Load sample JSON, then the Preview renders as it did before and the DATA tree is the same.
- Given a sample is saved, when the save finishes, then the document's revision, title and saved state are unchanged.

## Implementation Notes

- `jsonSampleFileFormat` (`src/file/file-access.ts`) is the single format value, and
  `sample-file.ts`'s open picker type is now DERIVED from it rather than re-spelled, so the
  open and save wordings cannot drift ("keep one wording for both").
- It is deliberately NOT in `knownFileFormats`, the set whose trailing extension
  `localFileName` strips. Membership there is a claim that the suffix is this application's
  own output and may be replaced; `.json` is the author's data file. Adding it silently
  changed SHIPPED naming on the other two save paths — a template titled `data.json` (the
  download tier's open input accepts `application/json`, and the title comes from the file
  name) was re-offered as `data.folio` instead of `data.json.folio`, and likewise for PDF.
  The sample save needs no strip: its own suffix takes `localFileName`'s case-preserving
  `endsWith(wanted)` early return. Three rows in `file-access.test.ts` now pin all of this.
- The panel's Save control answers to a new `saveDisabled` prop, NOT to the existing
  `busy`/`available` pair. Those two describe the sample OPEN picker (`sampleFileAccess`,
  `sampleBusy`); the save's conditions are the file boundary's (`fileBusy`, `fileAccess`),
  and a shell can have one without the other. Folding them together would have meant
  disabling Load sample JSON during unrelated file work — a change to how samples are
  loaded, which this story forbids.
- `sampleSaveInFlight` is a third in-flight latch beside `saveInFlight` and
  `exportInFlight`, for the reason story 13.1 minted the second: a third file, and the
  rendered `disabled` attribute is an ordering property of React's flush rather than an
  invariant of the handlers. All three handlers now refuse while any of the others is live.
- The two buttons sit in a `.data-actions` flex row with a `gap` (DESIGN.md:455-456), each
  at its natural width — the evidence rail's `flex: 1 1 0` is deliberately not inherited,
  so the shipped Load control's appearance is unchanged when it stands alone.

- **Review patches (triage rows 1–5):** `jsonSampleFileFormat` left out of `knownFileFormats` with three naming rows pinning it; the AC-1 round trip (saved bytes re-accepted give the same tree and `truncated`); cross-latch tests against the template save and the PDF export; the sample open picker's argument asserted; an App-level disabled case with no `fileAccess`.
- **Post-patch verification:**
  - `npm run build`: 0, including `verify:offline`.
  - `npx vitest run`: 1863 passed in 86 files.
  - `typecheck`: 0. `test:e2e:compile`: 0. `lint`: 0, with the 8 pre-existing warnings unchanged.
  - `npx playwright test`: 136 passed (6.6 min).
- **Manual check** (production build, 1440×900): Invoice opened from the startup dialog, DATA tab — Replace sample JSON and Save sample data sit side by side, the save enabled, above `invoice.sample.json`.

## Spec Change Log

## Review Triage Log

| # | Layer | Finding | Verdict | Evidence | Route |
|---|---|---|---|---|---|
| 1 | blind, edge, verification-gap | `jsonSampleFileFormat` joining `knownFileFormats` changed template and PDF save names ending in `.json` | medium | `localFileName` strips one known extension, so a title `data.json` saved as `.folio` went from `data.json.folio` to `data.folio`; titles come from opened files and the download tier's open input accepts `application/json`. No row of `file-access.test.ts` covers it | patch |
| 2 | blind | Acceptance criterion 1 (a saved sample reloads to the same data) is covered nowhere | medium | The e2e compares written bytes with loaded bytes; nothing feeds the saved bytes back through `acceptSampleData` | patch |
| 3 | blind, verification-gap | The new `sampleSaveInFlight` clauses in `save` and `exportPreviewPdf` are untested | medium | Pre-verified: deleting either clause leaves the suite green; the in-flight test presses only the sample control | patch |
| 4 | verification-gap | The derived sample open-picker type is asserted nowhere | medium | Pre-verified: both `sample-file.test.ts` tests ignore the picker argument, and e2e sample loads run on the input tier | patch |
| 5 | blind | The panel's disabled test is tautological and never maps `fileBusy`/`!fileAccess` to `saveDisabled` | low | A disabled button cannot call its handler in jsdom; no App-level case renders without `fileAccess`; direct test addition | patch |
| 6 | blind, edge, verification-gap | The control is disabled with no stated reason when there is no local file access | low | Real deviation from DESIGN.md's disabled-states-its-reason rule, but the frozen spec fixes both the control's disabled conditions and the panel's markup; the fix would edit this build's spec | rejected |
| 7 | blind, edge | Load/Replace sample JSON stays live during a sample save, so the bar can name a sample no longer loaded | low | The record is read once, so the bytes stay honest; only the status wording lags, and the panel shows the new file | rejected |
| 8 | blind, edge | `saveSampleData` does not refuse while `fileBusy` (Open, New…, Start blank) | low | Same shape as `exportPreviewPdf`, the shipped precedent; the control is disabled on `fileBusy` | rejected |
| 9 | blind | No `aria-describedby` from the control to the bar's outcome message | low | The frozen spec puts reporting in the document bar, as PDF export does; same relationship as the shipped export control | rejected |
| 10 | blind | The new describe re-spells the sample fixture instead of deriving bytes from it | low | A drift makes byte equality fail loudly, not silently | rejected |
| 11 | blind | `.data-actions` is a bare div rather than a labelled group, and comments cite DESIGN.md line numbers | low | Two buttons that already read as themselves; line citations match the existing habit in these files | rejected |
| 12 | blind | `record.bytes` is passed to `writeSave` without a copy | low | Both shipped tiers only read the buffer; a transferring tier is not demonstrated, and the guard adds a copy per save | rejected |
| 13 | edge | A sample name longer than 120 characters is bounded and carries an ellipsis into the suggested file name | low | Requires a >120-character file name; the author edits the name in the picker | rejected |
| 14 | edge | No check that `acquireSaveTarget` returned the requested format | low | Both tiers echo the requested format; speculative | rejected |

## Verification

**Commands:**
- `cd folio-designer && npm run build` — expected: exits 0, including `verify:offline`.
- `cd folio-designer && npx vitest run` — expected: pass.
- `cd folio-designer && npm run typecheck && npm run test:e2e:compile && npm run lint` — expected: pass.
- `cd folio-designer && npx playwright test` — expected: pass.

**Manual checks:**
- In the production build, open Invoice from the startup dialog, save its sample, reload, then load the saved file and confirm the Preview matches.
