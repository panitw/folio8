---
title: 'The setting governs what a save writes'
type: 'feature'
created: '2026-09-21'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: '20ebdb04ec8c3958e5d344eb4ef88e981376d156'
context:
  - '{project-root}/_bmad-output/specs/spec-font-sources-and-embedding/SPEC.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Three things are each complete and the product still cannot do what this epic exists
for. Story 4's setting is inert. Story 5's acknowledgement can be honoured but not produced — every
designer call site passes `false` and `StoredFace` has no such field, so a face the author imported
and acknowledged is refused when embedded. And a document that declines to embed has no way to say
so in its chains.

**Approach:** Give the setting teeth at the moment a face is chosen, carry the acknowledgement from
the store to the wire, and let an author who turns embedding off strip the faces the document
already carries — as one undoable act, after being told.

## Decisions

- **D1 — The setting governs the EMBED GESTURE, not a save-time transform.** Embedding is
  synchronous with the author's action today (`dispatchEmbed`, the cut fragments), and nothing is
  deferred to save. With embedding off, the gesture writes a **name** entry and embeds nothing,
  using the commands that already exist — `addFontChain` / `addFontChainEntry` take a bare face
  string, and `FontChainEntryAsk` structurally has no `asset` arm, which Go enforces at
  `component_commands.go:5397-5401`. Turning save into a transform would be a rewrite of a seam this
  story has no reason to touch.
- **D2 — The name written is the store's family-plus-style key.** The same rule `fontdir.Set` uses
  and story 3's import applies. This is the contract that makes a name-only document resolve against
  a host's directory; three implementations, one rule.
- **D3 — The strip is rewrite-then-drop, as ONE undo unit.** Replace each asset chain entry with its
  name entry, then let the existing `dropUnnamedFontAssets` remove what nothing names. Both
  mechanisms already exist. It goes through `applyCommands` / `commandUnitBytes` so the whole strip
  is one undo entry — a half-stripped document is not a state the author should be able to reach.
- **D4 — `StoredFace` gains the acknowledgement, and `soundFace` tolerates its absence.** The store
  is probed by shape rather than by version, and existing v2 records carry no such key — a strict
  check would drop every legacy record as corrupt, which is precisely the defect story 3 found and
  fixed. No version bump, no migration.
- **D5 — The author is warned before the first strip, and only the first.** Stripping is
  destructive and recoverable only through document undo.

## Boundaries & Constraints

**Always:**
- The three designer call sites derive the acknowledgement **from the face**, never from a literal
  and never by re-inferring it from the `source` text — the `TODO(story 6)` notes at
  `App.tsx:2714-2723`, `:3282` and `:3328` say so, and D3 of story 5 forbids two sources of truth
  about one face.
- A **pinning test** must exist: a stored-tier embed of a blank-terms author-supplied face asserting
  `authorAcknowledged: true` on the wire. Without it the gap lives in a story file instead of in CI,
  which is how it would survive.
- Embedded mode is untouched: byte identity, dedupe by SHA-256, faces stored whole, no save-time
  subsetting. This story adds a second mode; it does not alter the first.
- **No filesystem path, URL or machine identity** reaches the document — not as a hint, not as a
  comment. A non-embedded face is a face NAME.
- The preview must not move: toggling changes what the file contains, never the page set the
  designer previews. The designer resolves from its own store either way.

**Never:**
- No save-time transform, no deferred embedding, no save-time garbage collection of assets —
  `serialize.go` preserves orphans unconditionally for the `Parse(Serialize(d))==d` fixed point, and
  that is not this story's to change.
- No new chain-entry shape; no command that converts an asset entry to a name entry in place beyond
  what D3 composes from existing commands.
- No change to `fonts.Shipped()`, the catalogue tier, or the licence gates.
- Do not warn on every strip (D5), and do not strip without warning at all.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|---|---|---|---|
| Embed on, pick a family | Setting true | Face embedded, asset chain entry — exactly as today | N/A |
| Embed off, pick a family | Setting false | Name chain entry, nothing added to `assets` | N/A |
| Embed off, first use of a cut | Setting false, author presses **B** | Name entry for the bold cut, nothing embedded | N/A |
| Acknowledged face embedded | Author-supplied face, blank terms, setting true | Embeds — `authorAcknowledged: true` on the wire | N/A |
| Catalogue face embedded | Catalogue face, setting true | `authorAcknowledged: false`, as today | N/A |
| Turn off, document carries faces | Author sets false, faces present | Warned first; on accept, entries become names and the assets are dropped, as one undo entry | N/A |
| Decline the warning | Author cancels | Setting unchanged, nothing stripped | N/A |
| Undo a strip | Strip accepted, then undo | Faces and entries both return, in one step | N/A |
| Second strip | Author toggles off again later | No warning the second time | N/A |
| Legacy stored face | A store record written before this story | Loads and is usable; not dropped as corrupt | N/A |
| Preview unchanged | Same document, setting toggled | The designer previews identically | N/A |

</frozen-after-approval>

## Code Map

- `folio-designer/src/App.tsx:2713` `dispatchEmbed` -- the single embed dispatch
  (`embedFontFamilyCommand` → `sendFontChain(..., {action:'embed'})`), computing the fallback tail
  via `proposedFallbackTail`. The `TODO(story 6)` notes are at `:2714-2723`, `:3282`, `:3328`.
- Gestures that embed: **A** family pick, `addFamilyToDocument` → `installFamily` (degraded path at
  `:2999`); **B** applying an installed family, `embedInstalledFamily` at `:3151` (refetch path
  `:3120-3150`); **C** first use of a cut — plans built and one `embedFontCutFragment` pushed per
  plan (`:3284` catalogue bytes, `:3330` store-held), sent with
  `updateComponentPropertiesFragment` as one unit at `:3341` via `commandUnitBytes`.
- `folio-designer/src/font-chain-command.ts` -- the name-entry commands D1 uses:
  `addFontChainCommand(name, entries)` `:93`, `addFontChainEntryCommand(name, at, face)` `:105`;
  `FontChainEntryAsk` at `:70` is `string | {face, bold?, italic?, boldItalic?}` with **no `asset`
  arm**. Go handlers `addFontChain` `component_commands.go:4400`, `addFontChainEntry` `:4504`, and
  the enforcement at `:5397-5401`.
- `folio-go/component_commands.go:4606-4614` `dropUnnamedFontAssets` -- the strip's second half
  (delete at `:4611`), already called from `removeFontChainEntry` (`:4572`) and `deleteFontChain`.
  `assetKeyReferenced` is the predicate; `EmbeddedAssetKeys()` yields every key an entry names, base
  and cuts. Rationale for keeping orphans at save is at `:4576-4586`, pinned by
  `internal/template/assets_test.go:110,117` — **do not add a save-time GC**.
- `folio-go/component_commands.go:509` `applyCommandUnit` (kind `applyCommands` `:397`, 3 top-level
  fields, 1..64 members `:415-417`, no nesting `:530`) -- applies to a parsed copy of a snapshot, so
  it is all-or-nothing and therefore one undo entry. Designer side:
  `src/command-json.ts:148` `commandUnitBytes`, whose only caller today is `App.tsx:3341`.
- `folio-designer/src/font-store.ts:307` `StoredFace`; `soundFace` at `:420` (identity fields
  non-empty, licence trio may be `''`, key must match `storedKeyShape`); `databaseVersion = 2` at
  `:112`, with stores probed **by shape, not version** at `:100-106,:182`. `list()` `:711` and
  `get()` `:664` both route through `soundFace` — D4's tolerance matters at both.
- `folio-designer/src/font-import.ts:115` `authorSuppliedFaceSource(today)`, stamped by
  `acknowledgedFace` at `:130` -- what marks a face author-supplied today. It is **text**; D4 adds
  the boolean so nothing has to parse it back.
- `folio-designer/src/App.tsx:7534` `CompleteFontsDialog` -- the closest dialog to copy for D5: a
  count-bearing confirm/decline modelled on `UnsavedChangesDialog`, sharing `.page-dialog` styling
  and `role="dialog"` from `DeletePageDialog` (`:7418`).
- Save path: `App.tsx:4099-4108` (`engine.request('serialize', …)` then `fileAccess.writeSave`);
  engine `folio-go/serialize_template.go:19`. `embedFonts` is emitted only when `false`
  (`internal/template/serialize.go:155-164`), parse default `true` (`parse.go:64-69`); the setting's
  sole writer is `setDocumentEmbedFonts` at `component_commands.go:3478`.

## Tasks & Acceptance

**Execution:**
- [x] `folio-designer/src/font-store.ts` -- `authorAcknowledged` on `StoredFace`, with `soundFace`
      tolerating its absence on records written before this story (D4).
- [x] `folio-designer/src/font-import.ts`, `App.tsx` -- the import writes it; the three call sites
      derive it from the face and the `TODO(story 6)` notes are removed.
- [x] `folio-designer/src/App.tsx` -- with embedding off, each embed gesture writes a name entry
      through the existing chain commands instead of embedding (D1, D2), for all three gestures.
- [x] `folio-designer/src/App.tsx` -- the strip: rewrite entries to names, drop the assets, one
      `commandUnitBytes` unit (D3), behind the warning dialog (D5).
- [x] tests -- one per I/O Matrix row, **including the pinning test** for an acknowledged blank-terms
      face reaching the wire as `true`.
- [x] `docs/` -- say what the setting does now that it does something, twins in lockstep.

**Acceptance Criteria:**
- Given embedding is off, when the author applies a family to a chain, then the chain entry is a
  face name, `assets` gains nothing, and the designer previews in that face.
- Given an author-supplied face whose binary declares no licence records, when it is embedded, then
  the command carries `authorAcknowledged: true` and the engine accepts it.
- Given a document carrying embedded faces, when the author turns embedding off and accepts the
  warning, then every asset chain entry becomes a name entry, the assets are gone, and a single undo
  restores both.
- Given the author declines the warning, then the setting is unchanged and nothing is stripped.
- Given a store record written before this story, when the designer lists faces, then it is present
  and usable.
- Given the same document with the setting toggled, then the designer's preview is identical.
- Given `go test -count=1 ./...` in `folio-go` and `npm test` in `folio-designer`, then both pass.
  (`internal/text`'s `P6g` is pre-existing and red at baseline.)

## Implementation Notes

**The acknowledgement now travels on the FACE, not on the call site.** `ResolvedFace` in `App.tsx`
gained `authorAcknowledged`, so every producer states it once where the face is built — `false` at
each catalogue tier (fetch, refetch, committed-cache read), `held.authorAcknowledged` on the store
read — and the three former `TODO(story 6)` sites simply read it off the object. `keepOnThisMachine`
takes a `ResolvedFace`, so the import path needed no change at all beyond `acknowledgedFace`
stamping the flag beside `source`.

**Gesture A's name arm is the DEGRADED path only.** With a store present, applying a family sends no
command — the chain is written at first use (gesture B) — so `dispatchEmbed` is the single seam both
gestures reach and the fork lives there, once.

**A name entry DECLARES the cuts this machine holds (`namedFaceEntry`), and that is a deliberate
reading of D1.** `addFontChainEntry` writes a bare face and there is no command anywhere in the
vocabulary that adds a variant to an existing entry, so an entry that did not declare its cuts at
the moment it was written could never gain them — a name-only document could never bold at all. The
cuts are named, never carried; `addFontChain` still has no `asset` arm, and nothing reaches `assets`
on this path.

**A press that needs a cut the entry does not declare REBUILDS THE CHAIN; it does not append a
sibling entry.** The engine selects a weight from the `bold`/`italic`/`boldItalic` field of the entry
that covered the rune (`internal/template/model.go`'s `Variant`), never from a later entry — so an
appended `Brand Grotesk Bold` entry would draw bold in the base face, leave `chainDeclaresCut` still
answering "missing" so every press appended another one, and change the chain's glyph fallback order
besides. No command edits an entry's variants, but `addFontChain` takes `FontChainEntryAsk`, which
carries them, so `nameModeCutUnit` declares the cut by rebuilding: `renameFontChain` parks the chain
aside (carrying its elements), `addFontChain` declares it again with the cut on the base entry,
`updateComponentProperties` brings the elements back, `deleteFontChain` drops the parked one. Four
existing commands in one `applyCommands` unit — atomic, one undo entry, and no reachable state in
which the chain is parked, duplicated or missing. It refuses rather than guesses when the chain
carries a face (`addFontChain` has no `asset` arm) and fails atomically if a referrer this
projection cannot see — a table's `headerStyle.fontFamily` — still names the parked chain.

**Two consequences worth a later reader's attention, both stated rather than hidden:**

1. **A STRIPPED entry loses its declared cuts.** An asset entry's `bold`/`italic`/`boldItalic` are
   asset keys, and the strip's rewrite goes through `addFontChainEntry`, which writes a bare name.
   So a stripped document draws bold runs in its base face until the author asks for that weight
   again — which in name mode redeclares it by name through the rebuild above. This is said in the
   warning the author accepts and in `docs/folio-format.md`, not only here.
2. **The CANVAS still paints a named non-shipped face in the stylesheet's fallback stack.** The
   engine side of "the preview must not move" IS closed: `App.tsx` hands the engine every
   non-shipped face the document NAMES, read out of the machine store, through the existing
   `install-face` operation — so the rendered page set (and the PDF preview) is identical either
   way. The on-screen canvas asks a different question (`carriedFaces.has(fragment.assetKey)`), and
   answering it for a named face needs a face-name → asset-key map threaded through
   `CanvasComponent`. Not done; the canvas degrades to the declared stack, as it already does for
   any face it cannot name.

**`isShippedFaceName` is a SHAPE test, not a membership test**, and using it to decide "does the
engine already hold this face" answers yes for every brand typeface an author imports. A real
membership test over the `fonts.Shipped()` mirror was added as `isShippedFace` in
`shipped-face-cuts.ts`; the two are now distinct and each says which it is.

**The strip goes BEFORE the setting it belongs to.** A refused strip then leaves a document that
still carries its faces and still declares that it carries them — consistent, with nothing to roll
back. The other order leaves `embedFonts: false` on a document carrying every asset, and the setting
command has already been accepted as its own history entry, so there is no rollback available.

**The strip is planned before the author is asked, and re-checked after.** The plan is made of chain
positions, and the question is an await the author controls, so `applyPageSetup` captures the
snapshot revision before the dialog and refuses if the document was edited while it was open —
`documentGeneration` only catches a REPLACED document. The `applyCommands` 1..64 member bound is
checked at plan time for the same reason: discovering it after acceptance would refuse a destructive
act the author had already consented to.

**A carried face is named only when its record states BOTH halves of the key.** The projection seeds
`Family = AssetKey` for a record with no family (`page_setup.go`), so an emptiness test would be dead
code and a real such document would get a SHA-256 written into its chain as a face name; an empty
`style` would be read as the family's Regular, which on a document carrying only a Bold names a face
it does not hold. One rule, refusing in both cases, because refusing destroys nothing.

**A strip that would rename a face nothing here can supply is allowed but SAID.** The warning counts
the faces that are neither shipped nor held on this machine, because renaming one moves the preview,
which the Boundaries forbid doing silently.

**`stripWarned` is per document and armed on SUCCESS.** A session ref would strip every later
document of the session in silence; arming on acceptance would let a strip the engine refused
consume the one warning that document gets. `installDocumentIdentity` resets it and also clears the
pending question and resolves its promise, exactly as it does for the completion dialog.

**The strip is insert-then-remove, per chain.** `removeFontChainEntry` refuses to empty a chain, so
removing first would refuse every chain whose only entry is the embedded face — the ordinary case.
Inserting the name at the entry's own index and removing the shifted asset entry at `index + 1`
leaves every later index in that chain unmoved. The assets are dropped by the engine's own
`dropUnnamedFontAssets`, which `removeFontChainEntry` already calls, so an asset a second chain
still names is retained by the engine's predicate and not by a rule the designer keeps.

**Declining re-seeds the draft.** Skipping the embed row alone would leave the checkbox reading
"off" on a document that still embeds — the panel reporting a setting the author had just declined
to make. The other rows of the gesture still apply: the margins they typed are a different change.

**`embeddedChainBase`'s name arm takes entry ZERO or nothing.** A chain can be mixed — undo a strip,
or open an `embedFonts: false` document that carries assets — and `findIndex` over name entries
would pick the script fallback, so a cut would be declared against the Thai entry rather than the
one the text renders in.

**The install-face effect remembers a failure and says so once.** An un-remembered refusal would
re-read those bytes on every projection change with nothing anywhere saying why the preview is drawn
in another face. The record is cleared on document replacement and is scoped to the engine it was
made against, so it can never claim a face is held by a worker that is gone.

**No Go change was needed or made.** Every command this story sends already existed.

## Spec Change Log

## Review Triage Log

Pass 1 — three layers. The heaviest review of the six stories.

| # | Finding | Verdict | Evidence |
|---|---|---|---|
| 1 | In name mode a **B** press appends a *sibling chain entry* instead of declaring the cut on the base entry. Bold still renders in the base face; `chainDeclaresCut` keeps reporting it missing so the press repeats and duplicates accumulate; and the entry silently changes glyph fallback order for the whole chain. The Implementation Notes claim the opposite. | **high** | Confirmed by all three layers. The engine selects bold from the entry's `Bold` field (`model.go:232-247`), never from a later entry. |
| 2 | A refused strip leaves the document lying about itself: `setDocumentEmbedFonts` is committed *before* the unit, so on refusal the document declares `embedFonts: false` while still carrying every asset. No rollback, no test. | **high** | Confirmed. Reachable via #4's bound. |
| 3 | The strip unit emits two members per carried entry against `applyCommands`' 1..64 bound, with no check at plan time — a document with 33+ carried entries produces a unit the engine refuses whole, after the author has already accepted. | **medium** | Confirmed against `component_commands.go:415-417`. |
| 4 | The dialog promises "One undo puts everything back" while the code's own comment says the setting and the strip are deliberately **two** history entries. Following the dialog's advice reaches #2's inconsistent state. | **medium** | Confirmed. A false statement to the author, in the one dialog whose job is informed consent. |
| 5 | The unnamable-face guard is **dead code**: it tests `family === ''`, but the engine's projection sets `Family = entry.AssetKey` when no record family exists (`page_setup.go:202-217`). So a real such document writes a **SHA-256 hash into the chain as a face name**, which no host directory can resolve. | **high** | Pre-verified. The existing test passes only because it hand-builds a projection shape the engine cannot emit. |
| 6 | Stripping a face this machine does not hold renames it into a name nothing can resolve, moving both preview and render — the one thing the Boundaries say must never happen. | **medium** | Confirmed: the rename is unconditional, while `install-face` only covers faces in the store. |
| 7 | `stripWarned` is per-**session**, never reset on document replacement, so after one warned strip every later document in that session is stripped destructively with no warning. | **medium** | Confirmed. D5 reads as per-document. |
| 8 | A stale strip dialog survives a document change: `installDocumentIdentity` clears the completion dialog's state but not `stripRequest`/`stripAnswer`. | **medium** | Confirmed. The author answers a question about a document that is gone. |
| 9 | `embeddedChainBase`'s mode flag mis-picks on a **mixed** chain (asset + name entries), which is reachable by undoing a strip — an I/O row this story lists — or by opening a `embedFonts: false` document that carries assets. Inverting the flag's arms fails no test. | **medium** | Pre-verified. No test mounts a mixed chain. |
| 10 | `isShippedFace`'s cut half is unverified, and a stored copy of a shipped cut would be installed over the engine's own face — dropping `row.bold/italic/boldItalic` fails no test. | **medium** | Pre-verified. Directly undermines "the preview must not move". |
| 11 | The strip plan is computed from a stale projection and applied several awaits later; only `documentGeneration` is re-checked, never the snapshot revision, so a chain mutation landing while the modal is open makes the planned indices point at the wrong entries. | **medium** | Confirmed. |
| 12 | `stripWarned` is armed on acceptance, so a strip that then fails consumes the one allowed warning and the next is silent. | **medium** | Confirmed. |
| 13 | Half-empty font record handled two ways: an empty `family` refuses the whole gesture, an empty `style` silently becomes `Regular` — which for a document carrying only a Bold cut names the wrong face. | **medium** | Confirmed. |
| 14 | Declining the strip skips the embed row but not the draft, so the checkbox keeps reading "off" on a document that still embeds. | **medium** | Confirmed. |
| 15 | The `install-face` loop has an empty `catch {}` and never clears `installedNamedFaces` on document or engine replacement — a rejected face is retried silently forever, with no diagnostic. | **medium** | Confirmed. |
| 16 | Two I/O rows have no test: "Undo a strip" (shape is asserted as a proxy for atomicity, no undo is driven) and "Catalogue face embedded" (the negative half of the pinning test). | **medium** | Confirmed against the new test file. |
| 17 | The docs say turning the setting off "rewrites those entries as their names" without saying the rewrite **discards the entry's declared cuts** — so a reader deciding whether to turn it off is told the operation is name-preserving when it is not. | **medium** | Confirmed in both twins. |
| 18 | The strip drops an entry's `bold`/`italic`/`boldItalic` asset keys rather than renaming them. | **medium** | Confirmed. Subsumed by #1's fix. |

**Routing.** No `bad_spec` and no `intent_gap` — #1 initially read as one, because the frozen matrix
row "name entry for the bold cut" looked unreachable under the "no new command" boundary. It is
reachable: `FontChainEntryAsk` accepts `{face, bold?, italic?, boldItalic?}`, so the cut can be
declared by rebuilding the chain inside one `applyCommands` unit. All 18 route to **patch**.

## Design Notes

D1 is a correction to this epic's own SPEC.md, which says of CAP-2 that *"toggling the option and
saving changes only whether the referenced faces appear in `assets`"*. That phrasing assumes a
save-time transform. The designer does not work that way: a face is embedded the moment the author
picks it, so the setting must act at that moment instead. The observable promise CAP-2 makes still
holds — a document authored with the setting off carries no faces, one authored with it on carries
them — but it is reached at the gesture rather than at the write.

The strip is the one genuinely destructive act in this epic, which is why it is one undo unit and
why it is announced. Two things it must not do: orphan an asset another entry still names (which is
why the drop goes through the existing reference predicate rather than a new one), and leave a
document half-rewritten if a member is refused (which is what `applyCommands` buys).

## Verification

**Commands:**
- `cd folio-go && go vet ./... && go test -count=1 ./...` -- expected: pass except the pre-existing
  `internal/text` `P6g` corpus floor.
- `cd folio-go && go test -count=1 -run 'TestTargetRenderHash' -tags=matrix .` -- expected: pass.
- `cd folio-designer && npm test` -- expected: pass.
- `cd folio-designer && ./node_modules/.bin/oxlint && ./node_modules/.bin/tsc -p tsconfig.app.json
  --noEmit` -- expected: both exit 0. Run the binaries directly.
