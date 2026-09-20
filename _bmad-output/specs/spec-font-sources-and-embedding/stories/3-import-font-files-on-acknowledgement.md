---
title: "Import font files from the author's own machine, on their acknowledgement"
type: 'feature'
created: '2026-09-21'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: 'e2e443c18aaf20dc51ab016fd9abd1545c2ad7f0'
context:
  - '{project-root}/_bmad-output/specs/spec-font-sources-and-embedding/SPEC.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** A face reaches a Folio document by two routes — the eleven the engine ships, or one
fetched from the catalogue. An author whose company licenses a brand typeface has neither, and
`spec-fonts` D-8.6.1 declined disk import because such a file arrives with no licence terms and the
designer would have to invent them. The owner has reversed that premise: holding the right licence
is the author's responsibility, not this product's.

**Approach:** The author picks font files from their own machine, accepts an acknowledgement that
admits them, and the faces enter the designer's local face store grouped into one family by their
own name records — indistinguishable from a catalogue face everywhere downstream. Licence identity
is transcribed verbatim from each binary. Nothing is embedded in the document in this story.

## Decisions

- **D1 — Follow the image-file seam exactly.** `src/image-file.ts` already solves "let the author
  pick a file" twice over — `FileSystemImageAccess` (`showOpenFilePicker`) and `InputImageAccess`
  (`<input type="file">`) — chosen by `selectImageFileAccess` in `src/file/capability.ts` and
  injected as an `App` prop. Fonts get the same two-tier shape, the same capability selection and
  the same prop injection. No new pattern.
- **D2 — Key a face exactly as `fontdir` keys it: name ID 1, plus name ID 2 when the subfamily is
  not `Regular`.** This is not a style preference — it is the contract between this story and
  story 2. A document authored here names faces by these keys, and a host resolves them from a
  directory `fontdir.Set` built. If the two rules disagree the whole name-only path silently fails
  to resolve.
- **D3 — Licence records are transcribed, and absence is legal.** `copyright` from name ID 0,
  `licenceText` from name ID 13, `licence` from name ID 14, verbatim, with no classification and no
  inference. `faceCopyright` currently **throws** when name ID 0 is absent; that refusal does not
  apply to an author-supplied face, which stores an empty string instead.
- **D4 — The acknowledgement is one gesture per import, and it is the admission gate.** Not per
  file — an author importing four cuts answers once. Not remembered across imports — a checkbox
  nobody sees again is not an acknowledgement. Declining imports nothing: not a partial family, not
  a face held pending.

## Boundaries & Constraints

**Always:**
- Downstream of the import, nothing can tell an author-supplied face from a catalogue one: same
  store, same family browser row, same chain entry, same save/open round trip.
- Per-file checks stay **per file**, as they are for a fetched family: the `.ttf`/`.otf` media-type
  check and the variable-face refusal. One bad file is refused on its own — an author who picks
  four cuts and whose italic is a variable build gets the other three, and is told about the one.
- The face's `source` records that it came from the author's own machine and **carries no
  filesystem path**, no filename and no machine identity.
- `scripts/host-font-access.mjs`'s scan stays green and untouched. A file picker is not the Local
  Font Access API; `queryLocalFonts`, `navigator.fonts`, `'local-fonts'` and `FontData` remain
  banned.
- `src/file/file-access-contract.test.ts` stays green: no `showDirectoryPicker`, and no module
  other than `font-store.ts` opens IndexedDB.

**Never:**
- Nothing is embedded in the document, no chain entry is written, no engine command is sent, no
  `.folio` field is added. Stories 5 and 6 do that.
- Do not route an author-supplied face through `src/font-licence.ts`. That module decides whether
  terms a FAMILY PUBLISHES are terms this product accepts; it governs the catalogue tier and keeps
  its allowlist. An author-supplied face does not go through it.
- No classification, no inference from a family name, no defaulting to an identifier nobody read,
  no prompt asking the author to type terms.
- No drag-and-drop in this story, no directory picking, no network.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|---|---|---|---|
| Single face | One `.ttf` picked, acknowledgement accepted | One face in the store, keyed family-only if subfamily is `Regular` | N/A |
| Four cuts, one gesture | Four files of one family picked together | One family, four cuts, acknowledged once | N/A |
| Decline | Acknowledgement dismissed or cancelled | Nothing stored, nothing held pending | N/A |
| Mixed families | Files from two different families picked together | Grouped into two families by their name records, not into one | N/A |
| One bad file | Three good, one variable-font build | The three import; the variable one is refused by name | Per-file refusal, reported |
| Wrong type | A `.png` renamed `.ttf`, or a real `.png` | Refused by the media-type check, the rest still import | Per-file refusal, reported |
| No copyright record | A face whose name ID 0 is absent | Imports, with an empty `copyright` — no throw | N/A |
| No licence records | Name IDs 13 and 14 absent | Imports, with empty `licence`/`licenceText` | N/A |
| No family record | Name ID 1 absent or blank | Refused — it cannot be keyed | Per-file refusal, reported |
| Already held | A face whose bytes are already in the store | Not duplicated; the store is keyed by the SHA-256 of the bytes | N/A |
| Picker cancelled | The OS picker is dismissed with no file | Nothing happens, no dialog, no error | N/A |

</frozen-after-approval>

## Code Map

- `src/image-file.ts` -- **the pattern to copy.** `FileSystemImageAccess` (`showOpenFilePicker`,
  `:30`) and `InputImageAccess` (builds `input.type='file'`, `:46-56`); chosen by
  `selectImageFileAccess` in `src/file/capability.ts:41`; injected as an `App` prop at
  `src/App.tsx:346`; consumed in `applyImageAsset` at `:3541-3560`. `src/sample-file.ts` is the
  same shape for JSON. `src/image-file.test.ts:35,:45` shows how to drive a real `<input
  type="file">` and a `File` in a test.
- `src/font-store.ts` -- the store. `StoredFace` at `:307` already carries every field an imported
  face needs: `key` (lowercase hex SHA-256 of the bytes, `storedFaceKey` `:371`), `family`,
  `style`, `licence`, `licenceText`, `copyright`, `source`, `mediaType`, `scripts`, `fetchedAt`,
  `byteLength`. Writer is `FontStore.put` `:671-686`; the app-level wrapper that computes the key
  and stamps `byteLength`/`fetchedAt` is `keepOnThisMachine` at `src/App.tsx:3330`, and
  `recordFamilyCensus` at `:3366`. **No schema change should be needed** — confirm that.
- `src/font-name-table.ts` -- `nameTableString(view, tables, nameID)` at `:94` is generic and
  already handles platform 3/0 UTF-16BE with a latin-1 fallback. `faceCopyright` at `:151` reads
  name ID 0 and **throws at `:154`** when it is absent — D3 says that refusal does not apply here.
  `faceIsVariable` at `:198` is the variable-face refusal. **Nothing reads name ID 1 or 2 yet.**
- `src/font-source.ts` -- the catalogue path, for the shape to match rather than to change:
  `fetchWebFamily` at `:607`, `FetchedFace` at `:388`, media-type table at `:350` with
  `mediaTypeOf` at `:351` and the refusal at `:729`, variable refusal at `:782-783`, copyright read
  at `:785`.
- `src/App.tsx` -- `addFamilyToDocument` at `:2614` and `installFamily` at `:2809` are the
  catalogue flow this one runs beside. `FontBrowser` is mounted at `:4631` behind
  `fontBrowserOpen` (`:812`), opened from `FontFamilyProperty` at `:6584`.
- `src/FontBrowser.tsx:67` -- the Add font dialog; backdrop, modal and Escape trap at `:320` and
  `:154-159`. `src/font-browser-model.ts` -- `BrowserRow` at `:138` carries `cuts` at `:163`;
  `browserRows` `:193`, `cutLine` `:327`, `rowState` `:372`, `buttonLabel` `:378`.
- `src/App.tsx:7181-7200` `DeletePageDialog` -- **the confirm-dialog precedent**: `role="dialog"`,
  `aria-modal`, focus hold, Escape and Tab trap. Mounted at `:4543`.
- `src/font-provenance.test.ts` with `src/test/provenance-shape.ts` `assertProvenanceShape` -- a
  tripwire over the `source` field's shape, written for the two catalogue tiers. An author-supplied
  face is a third kind of `source`; this test must be extended deliberately, not bypassed.
- `scripts/host-font-access.mjs:56-60,:74` -- the four banned spellings and the scanned roots.
  `src/file/file-access-contract.test.ts:70` -- bans `showDirectoryPicker` and IndexedDB outside
  `font-store.ts`.

## Tasks & Acceptance

**Execution:**
- [x] `src/font-name-table.ts` -- add family and subfamily readers (name IDs 1 and 2) beside
      `faceCopyright`, and readers for name IDs 13 and 14; keep `nameTableString` as the one walker.
- [x] new module -- the two-tier font file access, mirroring `src/image-file.ts`, plus its
      selection in `src/file/capability.ts` and its `App` prop.
- [x] new module -- the import itself: per-file media-type and variable-face checks, name-record
      transcription, D2 keying, grouping into families.
- [x] `src/App.tsx` -- the acknowledgement dialog on the `DeletePageDialog` pattern, gating the
      import; and the store write through `keepOnThisMachine` / `recordFamilyCensus`.
- [x] `src/FontBrowser.tsx`, `src/font-browser-model.ts` -- the entry point for the import, and
      imported families listed exactly as catalogue families are.
- [x] `src/font-provenance.test.ts`, `src/test/provenance-shape.ts` -- admit the author-supplied
      `source` kind deliberately, with its own shape assertion.
- [x] tests -- one per I/O Matrix row, driving real `File` objects as `src/image-file.test.ts` does.

**Acceptance Criteria:**
- Given the catalogue is unreachable, when the author imports a `.ttf` from disk and accepts the
  acknowledgement, then the face is in the store and usable on a chain, and the designer previews
  in it.
- Given four cut files of one family picked in one gesture, when the acknowledgement is accepted
  once, then the family browser lists that family once with four cuts.
- Given the acknowledgement is declined, then nothing is written to the store and nothing is held
  pending.
- Given a face whose name ID 0, 13 and 14 are all absent, when it is imported, then it is stored
  with empty strings for those fields and no error is raised.
- Given a face file renamed on disk before picking, then its stored `family` and `style` are
  unchanged — they come from the binary — and its `source` contains no path or filename.
- Given `npm test`, `./node_modules/.bin/oxlint` and `./node_modules/.bin/tsc -p
  tsconfig.app.json --noEmit` in `folio-designer`, then all pass, including
  `host-font-access.test.ts` and `file/file-access-contract.test.ts`.

## Implementation Notes

**The Code Map's one open question, answered: a schema change WAS needed, and it is one line of
`soundFace`.** `StoredFace` carries every field an imported face needs and no field was added — but
`font-store.ts`'s `soundFace` held `licence`, `licenceText` and `copyright` to the same
present-and-non-empty rule as `key` and `family`. That rule is true of a face this product
distributes (`embedFontFamily` refuses without all three) and false of one the author supplies,
where an absent name record is legal. Measured, before the change: a face with no nameID 0/13/14
was written to the store and read straight back as a CORRUPT ENTRY and dropped — it vanished
between one listing and the next with nothing on screen. `soundFace` now splits its strings into
identity fields (still non-empty) and transcribed fields (must be strings, may be empty). No
version bump, no migration, and nothing an older record could have carried is now refused.

**Modules added.** `src/font-file.ts` (the two access tiers, `multiple: true`), selected by
`selectFontFileAccess` in `src/file/capability.ts` and injected as the `fontFileAccess` `App` prop;
`src/font-import.ts` (per-file checks, D2 keying, transcription, grouping, and
`authorSuppliedFaceSource` as the tier's one `source` writer).

**The census is written, and it is MERGED rather than replaced.** `familyIsComplete` reads a stored
family with no census as INCOMPLETE, which puts `+ Install` beside it in the font browser — and
pressing it would send `fetchWebFamily` after a brand typeface no catalogue publishes. So
`completeFontImport` writes one. `putCensus` is a plain put keyed by family, though, so writing a
fresh record over an existing one would SHRINK `published` to the cuts this machine happens to
hold and drop every recorded refusal with it — the family would read complete, `+ Install` would
disappear, and the cuts it still lacks would become unreachable. The written census therefore
unions the existing `published`, the cuts already held and the cuts this import added, and carries
the existing `refused` through untouched.

**A family name the catalogue already offers is refused at the import, by file.** `offeredFamilies`
gives one row per family and lets the committed catalogue's row win (`localTierHolds`), so a face
imported under a catalogue family's name would be written to the store, reported as kept, and
reachable from nowhere. Making it reachable instead would mean either two rows sharing one family
name — which the browser keys by family and the family control stages by family — or the stored row
displacing the catalogue row, which would take the release's own cuts of that family out of reach.
Both restructure `FamilySource`, which is outside this story. The refusal is per file, in the same
shape as every other per-file refusal, and says the one thing that is true: two different faces
cannot share one family name, and a face is named by its own binary so this one cannot be renamed.

**Two files declaring the same face name: the first picked wins, the second is refused by name.**
`fontdir.Set`'s own rule and its reason — determinism matters more than which one wins, and nothing
here can tell which of two `Bold` files the author meant.

**The media type is still read off the extension, and the refusal says so.** A valid static sfnt
named `BrandGrotesk.ttf.bak` is refused, and the sentence tells the author to rename it. The
alternative — deriving the media type from the sfnt version — was rejected because `fontdir` on the
host side reads it off the extension too (`fontExtensions`), so deriving it here would put the
designer and the renderer on two rules for one fact. The module's thesis that a filename is not
IDENTITY is untouched: the family and the cut still come from the binary.

**The acknowledgement day is stamped when the author answers, not when they pick.** `ImportedFace`
carries no `source` at all; `acknowledgedFace(face, today)` is the one writer, called from
`completeFontImport` with a date computed at that moment. A pick that straddles UTC midnight cannot
record a day nobody acknowledged anything on.

**The import is gated on `storeKeepsFaces`.** An import's only sink is the machine store, so in a
browser that cannot keep a face the control is not drawn and the footer says why — asking the
author to pick files and assert a right over them, only to refuse every write, is a question asked
for nothing.

**`scripts` is empty on an imported face.** Nobody has classified the binary, and inferring
coverage from a family name is the inference this story forbids. `browserRows` falls back to the
snapshot's scripts when the tier records none, and a family the snapshot never heard of carries no
script badge — which is true.

**Deliberately not done, per the Never list:** nothing is embedded, no chain entry is written, no
engine command is sent, no `.folio` field is added, `src/font-licence.ts` is not on this path, and
`rowTierNote`'s `downloaded to this machine` was left alone — the boundary requires an imported
family to be indistinguishable from a catalogue one in the browser row.

**Untouched, and verified green without being edited:** `src/host-font-access.test.ts`,
`src/file/file-access-contract.test.ts`, `scripts/host-font-access.mjs`,
`scripts/forbidden-font-hosts.mjs`.

## Spec Change Log

## Review Triage Log

Pass 1 — three layers.

| # | Finding | Verdict | Evidence |
|---|---|---|---|
| 1 | An import whose family name matches a catalogue family **overwrites that family's census**, because `putCensus` is a plain put keyed by family. `published` shrinks to the cuts on this machine, `familyIsComplete` then reads the family as complete, `+ Install` disappears and the remaining catalogue cuts become unreachable. | **high** | Pre-verified by the verification-gap layer. A regression of already-shipped behaviour, not new-feature risk: it disables the install-all-cuts path for that family. |
| 2 | An imported face whose family matches a **shipped** family is stored and reported as kept, but never appears as a browser row and can never be picked. | **medium** | Verified directly: `font-index.ts:486` does `if (localTierHolds(record.family)) continue`. The author's licensed cut of e.g. Roboto is swallowed by the shipped family of the same name. |
| 3 | The acknowledgement dialog's keyboard contract — Escape declines, Cancel holds focus, Tab cycles — is asserted nowhere; every case reaches it with `fireEvent.click`. | **medium** | Pre-verified: dropping the Escape branch leaves the whole suite green. The repo asserts exactly these things for `DeletePageDialog` (`App.test.tsx:11405`) and `UnsavedChangesDialog` (`:12337`). |
| 4 | The one behavioural change to `font-store.ts` — splitting `soundFace` into identity and transcribed fields — has **no store-level test**, and no end-to-end case imports a blank-record face and asserts it survives a later listing. | **medium** | `font-store.test.ts` is untouched. The two halves are each tested; the join, which is exactly where the measured bug lived, is not. |
| 5 | Two picked files declaring the **same family and the same cut** with different bytes both import; the family then holds two conflicting `Bold` faces with no refusal and no defined winner. | **medium** | Confirmed: grouping is by family only, and the SHA-keyed store gives them different keys. The matrix covers identical bytes but not this, which is the likelier author mistake (a v1 and v2 of one cut). |
| 6 | `completeFontImport` has no failure path of its own: a throw from `keepOnThisMachine`, `recordFamilyCensus` or `refreshStoredFaces` escapes as an unhandled rejection, the dialog vanishes and the author is told nothing. | **medium** | Confirmed. `finally` clears the busy flag but no message is ever set. |
| 7 | Admission still hinges on the **filename** — `mediaTypeOf(file.name)` refuses before a byte is read — in a module whose entire thesis is that filenames are not identity. A valid sfnt named `BrandGrotesk.ttf.bak` is refused though `requireStaticTrueTypeTables` would admit it. | **medium** | Confirmed. Reachable in the `<input type="file">` tier, whose `accept` is advisory. |
| 8 | The import control is not gated on `storeKeepsFaces`, so an author can pick files and acknowledge in a designer that cannot keep faces at all; every write then refuses. | **medium** | Confirmed at the button's `disabled` expression. |
| 9 | `fontImportMessage` is cleared only when a new import starts, so closing and reopening the browser greets the author with a stale report. Cancel is also not disabled while the import is writing. | **low** | Both confirmed. |
| 10 | `today` is defaulted when the files are picked, not when the acknowledgement is accepted, so a pick that straddles UTC midnight records the wrong day. | **low** | Confirmed. |
| 11 | The "N faces are now on this machine" count counts picked files, not distinct SHA-256 keys, so two byte-identical files claim two. | **low** | Confirmed. |
| 12 | `selectFontFileAccess` is a braceless `if` separated from its body by nine lines of comment, with a second `return` below that reads as dead code; the image sibling two lines up keeps condition and body together. | **low** | Confirmed. Correct today; any inserted statement silently becomes the `if` body. |
| 13 | `font-import.test.ts`'s control-character case contains a **raw BEL byte** in a string literal, so it reads on screen as asserting a clean `Sarabun` face is refused. | **low** | Confirmed with `od -c` by the reviewer. Invisible in source and diff; any formatter that strips it flips the test. |
| 14 | `font-import.test.ts`'s header points at `App.font-store.test.tsx` for behaviour that lives in the new `App.font-import.test.tsx`. | **low** | Confirmed. The wrong pointer is plausible enough to follow. |
| 15 | An imported face carries `scripts: []`, so such a family carries no script badge and vanishes from the writing-system filters. | **low — accepted** | Confirmed and deliberate: classifying scripts would be inference, which this story forbids. Recorded in `DECISIONS.md` A-20 rather than patched. |
| 16 | The Acceptance Criteria name `tsc -p tsconfig.test.json`, which does not exist in the designer; the Verification section says to use `tsconfig.app.json`. | **low — fixed by me** | True, and my error in both places. The Verification block is already corrected; the criterion is corrected with it. Not sent to the implementer. |
| 17 | An author imports a face declaring **no licence records**, and can then never use it on a chain: `component_commands.go:4914` refuses a font record whose `licence`, `licenceText` or `copyright` is blank, and `parse.go`'s `requireEmbeddedFaceLicence` refuses whitespace-only terms. | **high — routed to story 5** | Verified by reading both doors. This is a genuine cross-story gap I missed when planning: story 3's matrix deliberately admits such a face, and nothing embeds in story 3, so nothing here is wrong — but the promise only becomes true when story 5 relaxes that door for an acknowledged face. Recorded in `DECISIONS.md` A-19 and added to story 5's scope. |

**Routing.** No `bad_spec` and no `intent_gap` inside this story. #1–#14 route to **patch**; #15 is
accepted with its consequence recorded; #16 is mine to fix and is fixed; #17 is real and high but
belongs to story 5, which touches the exact door it names.

## Design Notes

D2 is the one decision that reaches outside this story, so it is worth restating why it is a
constraint and not a convention. Story 2's `fontdir.Set` keys a disk face `Sarabun` / `Sarabun
Bold` from name IDs 1 and 2. A document authored here will, in story 6, name faces rather than
carry them. If the designer keyed a face any other way — by filename, by PostScript name, by the
family record alone — the name a document carries would not be the name a host's directory
produces, and a name-only document would fail to resolve on exactly the setup this whole spec
exists to serve. The two rules must be the same rule.

## Verification

**Commands:**
- `cd folio-designer && npm test` -- expected: all pass.
- `cd folio-designer && ./node_modules/.bin/oxlint && ./node_modules/.bin/tsc -p
  tsconfig.app.json --noEmit` -- expected: both exit 0. Two traps here, both of which caught this
  spec's first draft: run the binaries DIRECTLY, because this environment's command proxy reports
  a spurious failure from `npm run lint`; and the designer's config is **`tsconfig.app.json`** —
  `tsconfig.test.json` is `folio-js`'s and does not exist here.
- Confirm `src/host-font-access.test.ts` and `src/file/file-access-contract.test.ts` are green
  **without being edited** — if either needed changing, the wrong thing was built.
