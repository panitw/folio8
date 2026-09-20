---
title: 'Embed a cut on first use'
type: 'feature'
created: '2026-09-19'
status: 'draft'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/specs/spec-install-all-face-cuts/SPEC.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The designer can put a family's whole face set on the machine (story 1) but has no way
to put a second face into a document. `embedFontFamily` writes one asset and one entry and refuses a
chain name it already holds (`component_commands.go:4374`), and its own comment says so — *"a pick
embeds ONE face"*. The engine's read side is already finished: `FontChainEntry` carries
`Bold`/`Italic`/`BoldItalic` (`model.go:197-221`), `EmbeddedAssetKeys()` walks them, parse.go
enforces their namespace, self-reference and licence rules, and the canvas already projects them.
Only the write door is missing. Meanwhile the cut-absence sentence reads the document's chain alone
(`App.tsx:5214`), so a cut sitting on the machine still reads as *"No bold face in this family"*.

**Approach:** Settle the wire contract for embedding one cut of an already-embedded family, and make
the warning consult both sources. First use of bold or italic on a family whose entry is embedded and
whose matching face is in the local store sends a cut embed — the face bytes, its own six-field
licence record, the target chain and the cut — which writes the asset and sets that one variant key
on the entry. Then the ordinary `bold`/`italic` property commit follows, as D-16.5 requires. The cut
absence sentence keeps its exact words but changes its subject: it now means *this family has no such
face*, never *this document has not embedded it yet*.

This story is planned **on a dirty working tree**, against uncommitted spec-deferred-offline-cache
story 5 work, at the owner's direction. It also **assumes story 1 has landed**: that the local store
holds a family's whole face set with a real cut name in each record's `style`, and that `FamilySource`
(or a sibling accessor) can answer *"the face record for family F at cut C"* synchronously from the
`storedFaces` array the panel already receives.

## Boundaries & Constraints

**Always:**
- The `.folio` format does not change. No new key, no doc edit — the parse-side rules this command
  must mirror are already written (`docs/folio-format.md:246-300`, `:955-962`).
- `SerialisesAsObject()` (`model.go:283`) stays the sole shape decider, and `fontsRequireMajor` keeps
  asking the same predicate, so a document that declares no variant is byte- and version-identical.
- A variant asset key clears the **same** admission bar as the base: six non-blank record fields via
  `embeddedFontRecord`, `RefuseVariableFace`, `RefuseContradictedLicence`, and the 512-char
  `maxCanvasPropertyString`.
- An embedded face is never replaced. The base Regular stays required and is never touched.
- `assetKeyReferenced` (`component_commands.go:1248`) must learn variant keys **in the same commit**:
  today it tests `entry.AssetKey` only, so a variant-only asset would read as unreferenced.
- A new Go wasm test must use the in-flight two-arg `NewEngine(testClock(), fonts.Shipped())`.

**Never:**
- No new diagnostic code; absence keeps `TEXT_STYLE_FACE_UNDECLARED` and base-face paint.
- No synthetic bold or oblique, no variable axes, no walking the chain for a bold elsewhere.
- No change to `chainFaceNames`, the render path, `CanvasFontChainEntry`'s key set, or
  `engine-protocol.ts`'s `hasExactKeys` list — variants are already projected.
- No prune-on-save and no document-wide asset sweep (refused by D-16.5).
- No re-shaping of story 1's store or index model, and no edit to the in-flight story-5 files beyond
  the two-arg `NewEngine` form.
- No second face for a family whose chain entry is a **face** entry (shipped/declared) — those carry
  FontSet face names, and `shippedFamilyEntry` already declares them at declare time.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| First bold | chain `Sarabun` = `[{asset: REG}]`; store holds Sarabun Bold | asset `BOLD` written with its own licence record; `fonts.Sarabun[0].bold = BOLD`; then `bold:true` commits; paints bold, no diagnostic | N/A |
| Cut already declared | `fonts.Sarabun[0].bold` already names those bytes | no asset, no chain change, canonical bytes do not move, no second history entry | N/A |
| Held but not embedded | store holds Sarabun Bold; chain declares no `bold` | no cut-absence sentence in the panel | N/A |
| Genuinely absent | store holds no Sarabun Bold | sentence stands verbatim; engine still warns `TEXT_STYLE_FACE_UNDECLARED` and paints Regular | N/A |
| Face fails admission | bytes carry `fvar`, or a contradicted licence, or a blank record field | document unchanged; located command refusal; panel shows a refusal sentence through `refuseFontChain`; the property is not committed | refusal names the family and the cut |
| Variant names its own base | cut bytes hash to the entry's `AssetKey` | refused, mirroring `parse.go:628-631` and `component_commands.go:4751` | located at `fonts.<chain>[<i>].<cut>` |
| Target is a face entry | chain entry is a bare face name | refused — a face entry's variants are FontSet face names, never asset keys (AD-8) | located at the entry |

</frozen-after-approval>

## Open Questions

1. **The command shape — widen `embedFontFamily`, or mint a second kind?** `componentFields` is an
   exact count, not a key-set check, so an optional argument is impossible; and the two operations
   have *disjoint* preconditions (a pick refuses a chain name that exists, a cut embed requires one).
   - **A. Widen `embedFontFamily` to 13 fields** with a `cut` field (`""` = today's pick). One kind;
     but the handler becomes a mode switch, and every 12-field literal moves —
     `embed_font_test.go`'s `embedFontCommand`, `font-chain-command.test.ts:52/128/181`,
     `component_commands_test.go:3222`.
   - **B. Mint `embedFontCut`** with its own exact arity (chain, cut, the six record fields,
     mediaType, data — no tail). `embedFontFamily` and its tests are untouched and each kind's
     `componentFields` count states its own contract; the cost is one case in the closed dispatch
     switch, one builder in `font-chain-command.ts`, and one arity row in its test.
   - *My read:* **B.** The dispatch note frames this as "extends `embedFontFamily`", but the measured
     tree says a new kind is the cheaper and more idiomatic change here. Flagging the divergence.

2. **How many undo entries is one press of B?** D-16.5 ruled a pick's embed and its property commit
   are two commands with two undo entries, never fused, and there is **no compound, batch or
   transaction command anywhere in the repo** — `Apply` decodes exactly one command object.
   - **A. Two entries (follow D-16.5).** One undo turns bold off and leaves the face embedded and the
     cut declared; a second undo removes it. An author who undoes once has a document carrying a face
     nothing currently uses — it stays *referenced* by the entry, so it is not an orphan, and
     prune-on-save is already refused.
   - **B. One fused entry.** Requires a new compound Go command kind — a materially larger change
     than this story, and it reopens a ruling D-16.5 made deliberately.
   - *My read:* **A.** This is SPEC.md's open question *"Does undoing it remove the asset from the
     document, or leave an unreferenced face behind?"* — the answer under A is: two undos remove it,
     one undo leaves it referenced but unused.

3. **A cut available only at a mismatched upstream vintage** (SPEC.md's own question, verbatim: *"The
   freeze rule says a later cut is taken at the embedded Regular's vintage where available. When it is
   not — the store holds only the newer bold — is the mismatched bold embedded, or is the cut treated
   as absent and the warning left standing?"*).
   - **A. Embed it anyway.** The document carries two faces from different upstream vintages. The
     freeze rule is still honoured — the embedded Regular is never touched — but the family's cuts are
     no longer one cohort.
   - **B. Treat the cut as absent.** No embed, warning stands; but then CAP-2's sentence would be
     saying "this family has no such face" about a face the machine is holding, which is the exact
     falsehood this story exists to remove.
   - *My read:* **A**, and note the measured premise: **a mismatch is not detectable today.**
     `StoredFace` is keyed by content hash alone and its only vintage field is `fetchedAt`, which is
     `YYYY-MM-DD` (`font-store.ts:135`, `font-index.ts:241`) — day-granular, so four faces installed
     the same day are indistinguishable. Choosing **B** obliges story 1 to record a per-install cohort
     id on every face; choosing **A** makes the detection question moot.

## Code Map

- `folio-go/component_commands.go` — `embedFontFamily` `:4280` (`componentFields(raw, 12)` `:4281`);
  reusable as-is: `embeddedFontRecord` `:4429`, `embeddedFaceBytes` `:4468`, `fontChainName` `:3979`,
  `boundedChainFaceName` `:4801`. Gates to copy in order: `RefuseVariableFace`, then
  `RefuseContradictedLicence` `:4363`, then the asset write. Stale comment to rewrite: `:4412-4414`.
  `assetKeyReferenced` `:1248` — must walk `EmbeddedAssetKeys()`. Self-reference mirror `:4751-4756`.
  Bound `maxCanvasFontChainEntries` `:3886`.
- `folio-go/internal/template/model.go` — **read-side is done.** `FontChainEntry` `:197-221`,
  `fontChainVariants` `:245-253` (closed: `bold`/`italic`/`boldItalic`), `Embedded()` `:270`,
  `SerialisesAsObject()` `:283`, `Variant()` `:304`, `EmbeddedAssetKeys()` `:321`. Do not change.
- `folio-go/internal/template/parse.go` — the rules the command door mirrors: namespace `:636-656`,
  self-reference `:628-631`, `requireEmbeddedFaceLicence` `:651`/`:749`. Do not change.
- `folio-go/internal/designer/page_setup.go:606-615` + `page_setup.go:222-230` — variants already
  projected and already bounded at 512. Do not change.
- `folio-designer/src/App.tsx` — `dispatchEmbed` `:2544`, `embedInstalledFamily` `:2815`,
  `refuseFontChain` `:2509` (returns the sentence; never throws), stale comment `:2535-2538`,
  `carriedFaceKeys` `:888` (already flat-maps variant keys — no prefetch change needed),
  `storedFaces` state `:672` **already passed into `ComponentProperties` `:4411` and used at
  `:4465-4467`**, `chainDeclaresCut` `:5214`, `missingCutFor` `:5240`, `selectionMissingCut` `:5255`,
  `cutAbsenceSentence` `:5277`, the B/I toggles at `:4474`.
- `folio-designer/src/font-chain-command.ts` — `FontChainEntryAsk` `:63`, `variantKeys` `:69`,
  `chainEntry` `:74-82` (absent cut = **absent key**, never `""`), `embedFontFamilyCommand` `:127-147`.
- `folio-designer/src/font-store.ts` — `StoredFace` `:122-138` (`style` `:126`, `fetchedAt` `:135`),
  `FontStore.get` `:157`, `storedFaceKey` `:182`. Story 1 owns the family/cut accessor.
- Existing fixtures that already prove the read side: `folio-go/style_face_embedded_test.go:110`,
  `internal/template/font_chain_variants_test.go:515` (`TestAVariantAssetKeyMustStateItsTerms`),
  `canvas_font_chain_entry_test.go:255`.
- Guards that will react: `component_commands_test.go:3222` (arity verdict),
  `font-chain-command.test.ts:22/126`, `command-json-soleness.test.ts:91` (factory list spelled
  **twice**), `canvas_projection_wire_test.go:113/390`, `internal/wasm/embed_font_test.go`.

## Tasks & Acceptance

**Execution:**
- [ ] `folio-go/component_commands.go` -- add the cut-embed door (kind and arity per Open Question 1) -- writes one asset and sets exactly one variant key on the chain's embedded entry, reusing `embeddedFontRecord`/`embeddedFaceBytes` and running the same three admission gates in the same order.
- [ ] `folio-go/component_commands.go` -- `assetKeyReferenced` walks `EmbeddedAssetKeys()` instead of `entry.AssetKey` -- a variant-only asset must not read as unreferenced; red-proof by asserting the old form returns false for a variant key.
- [ ] `folio-go/component_commands.go:4412-4414` -- rewrite the "a pick embeds ONE face / the entry declares no cut" comment -- it will be standing over code that no longer does what it says.
- [ ] `folio-designer/src/font-chain-command.ts` -- add the cut-embed builder beside `embedFontFamilyCommand`, using the same `quote`/field discipline -- the wire shape every later story builds on.
- [ ] `folio-designer/src/App.tsx` -- add a first-use cut embed beside `embedInstalledFamily`, wired from the B/I toggles through `ComponentProperties`, sending the cut embed and then the property commit -- reuse `refuseFontChain` for every anticipated refusal.
- [ ] `folio-designer/src/App.tsx` -- `chainDeclaresCut`/`missingCutFor`/`selectionMissingCut` consult `storedFaces` as well as the chain -- `storedFaces` is already a prop at `:4411`, so this is a signature change, not new plumbing.
- [ ] `folio-designer/src/App.tsx:2535-2538` -- rewrite the stale "THE PICKED ENTRY ITSELF DECLARES NOTHING" comment.
- [ ] `folio-go/component_commands_test.go`, `folio-go/internal/wasm/` -- cover every I/O matrix row at the command door, plus a history test (one revision, undo removes the variant asset and the key) using `NewEngine(testClock(), fonts.Shipped())`.
- [ ] `folio-designer/src/font-chain-command.test.ts`, `src/App` tests -- pin the new builder's exact arity and the warning's two sources, including the held-but-not-embedded row.

**Acceptance Criteria:**
- Given a document whose `Sarabun` entry is embedded and whose Bold is in the local store, when the author presses **B** for the first time, then the document gains exactly one asset carrying its own six-field licence record, `fonts.Sarabun[0].bold` names that key, and the text paints bold with no diagnostic.
- Given that same document, when the author uses the cut again, then no second asset is written, the canonical bytes do not move, and no second history entry is pushed.
- Given a document declaring no variant anywhere, when it is round-tripped, then its bytes and its version stamp are identical to before this story — `SerialisesAsObject()` is still the only shape decider.
- Given an embedded family the local store holds no Bold for, when the panel renders, then the cut-absence sentence appears with its words unchanged and the engine still emits `TEXT_STYLE_FACE_UNDECLARED`.
- Given a cut face that fails any admission gate, when first use is attempted, then the document is unchanged, the refusal is located, and the panel shows a refusal sentence rather than throwing.

## Implementation Notes

## Spec Change Log

## Review Triage Log

## Design Notes

**Byte neutrality is structural, not incidental.** `writeFontChain` asks `SerialisesAsObject()`, which
is `Embedded() || any variant non-empty`; `fontsRequireMajor` asks the identical predicate. A document
that declares no cut therefore emits the same bare string or the same one-key `{"asset": …}` and the
same version stamp. No golden should move. If one does, that is a finding, not a rebaseline.

**The sentence keeps its words and changes its subject.** `chainDeclaresCut` returns `true` (= no
warning) for an unknown family or a missing chain; the store check is an additional way to reach
`true`, never a way to reach `false`. Absence must remain a statement about the *family*, so the check
is "the local store holds a face for this family at this cut", not "this document has embedded it".

**Order is forced by the engine.** The cut must be embedded before the property is committed, for the
same reason a pick embeds before it applies: the engine refuses a property naming something the
document has not declared.

## Verification

**Commands** (run each separately — a conjunction drops everything after a known-failing term):
- `cd folio-go && go test -count=1 ./...` -- expect the mandated permanent red set only (`TestCorpusMeetsP6ExerciseFloors` + `P6g_(opaque_names)`); re-measure the pass count, never relay one.
- `cd folio-go && go vet ./...` -- exit 0.
- `gofmt -l /Users/panitw/Projects/folio/folio-go` from the **repo root** -- empty.
- `cd lint && go test -count=1 ./...` -- four `ok`; `-count=1` is mandatory, a cached `ok` is not a measurement.
- `cd folio-designer && npm test` -- all passing; report file/test counts.
- `cd folio-designer && npx tsc -b --force` -- exit 0. `npx oxlint` -- exactly 4 pre-existing `only-export-components` warnings.
- `cd folio-go && GOOS=js GOARCH=wasm go test -count=1 -exec="$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./wasm/cmd/engine/` -- the wasm-host boundary is invisible to `go test ./...`.
- Byte identity: `cd folio-go && go test -count=1 -run 'Golden|ByteIdentity|Fixture' ./...` -- no golden may move.
