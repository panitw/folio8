---
title: 'Embed a cut on first use'
type: 'feature'
created: '2026-09-19'
status: 'done'
route: 'dispatch'
baseline_commit: '9d33fbedb5bb4e5df801f6e446d941b9eab6e31f'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/specs/spec-install-all-face-cuts/SPEC.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Story 1 put a family's whole face set on the machine and gave it a census of what
upstream publishes; nothing can put a second face into a *document*. `embedFontFamily` writes one
asset and one chain and refuses a chain name it already holds
(`component_commands.go:4547`), and its own comment says so — *"a pick embeds ONE face"*. The
engine's read side has been finished since story 11.2: `FontChainEntry` carries
`Bold`/`Italic`/`BoldItalic`, `EmbeddedAssetKeys()` walks them, `parse.go` enforces their namespace,
self-reference and licence rules, and the canvas already projects them. Only the write door is
missing. Meanwhile the cut-absence sentence reads the document's chain alone
(`App.tsx:5399`) and says *"No bold face in this family"* about a bold sitting on the machine,
about a bold that merely failed to download, and about a bold that genuinely does not exist.

**Approach:** Mint `embedFontCut`, an engine command that attaches one variant asset key — with its
own licence record, through the same admission gates as the base — to an entry of a chain that
already exists. Pressing **B** on a family whose cut is held sends that command and the ordinary
`updateComponentProperties` inside **one `applyCommands` unit** (story 6), so the two travel as one
undo step without being fused into one kind. Make `applyCommands` reachable from TypeScript, which it
is not today. And split the cut-absence note into the three states the census can now tell apart.

Planned at `9d33fbe`, **after** story 1 (`9506503`) and story 6 (`e08f558`), on a clean tree.

## Decisions carried in

- **D-owner-1 — Pressing B is ONE undo entry**, delivered by story 6's unit. Embedding and
  property-setting stay two separate commands travelling inside one unit; they are never fused into
  a single kind. This also answers SPEC.md's *"does undoing it remove the asset"* — one undo removes
  both.
- **D-owner-2 — A mismatched upstream vintage is embedded anyway.** The freeze rule holds because
  the embedded Regular is never touched. A mismatch is not reliably detectable in any case: the
  store's only vintage field is `fetchedAt`, `YYYY-MM-DD` (`font-store.ts`).
- **D-owner-3 — The warning has three states.** *"No bold face in this family"* is for
  **upstream-publishes-none only**. A cut that exists upstream but could not be fetched gets its own
  sentence naming that, and a transient failure stays retryable. Story 1's census is the source; no
  second source may be invented for it.
- **D-owner-4 — AMENDED AT REVIEW: there is a FOURTH state, for "not known".** The three-state model
  assumed a census is always present. It is not: the v1→v2 store migration is purely additive and
  writes no census rows, a `putCensus` failure leaves the faces written and the census absent, and
  `installFamily`'s partial-write path refuses and returns **before** `recordFamilyCensus`. All three
  reach the panel with face records and no census. The fourth sentence must be **actionable and must
  claim nothing about upstream in either direction** — neither that the family has the cut nor that
  it lacks it. D-owner-3's reservation of *"No bold face in this family"* for upstream-publishes-none
  **only** is preserved exactly; that is what ruled out reusing it here.

## Boundaries & Constraints

**Always:**
- The `.folio` format does not change. No new key, no doc edit — the parse-side rules this command
  mirrors are already written (`docs/folio-format.md:246-300`, `:955-962`).
- `SerialisesAsObject()` stays the sole shape decider, so a document declaring no variant is byte-
  and version-identical to before this story.
- A variant asset key clears the **same** bar as the base: six non-blank record fields via
  `embeddedFontRecord`, `RefuseVariableFace`, `RefuseContradictedLicence`, the 512-char bound.
- **An embedded face is never replaced.** A cut already declared over *different* bytes is refused,
  never overwritten. The base Regular is never touched.
- **Permanence, never prose, decides retryability.** `FamilyCutRefusal.permanence`
  (`font-store.ts`) is the discriminant; no sentence may be parsed to infer it.
- The census has **no `held` field** and must not gain one — held-ness is derived from face records.
- `familyIsInstalled` ("can these bytes be used") and `familyIsComplete` ("is anything left to
  fetch") are two questions; neither may be substituted for the other.

**Never:**
- No new diagnostic code; absence keeps `TEXT_STYLE_FACE_UNDECLARED` and base-face paint.
- No synthetic bold or oblique, no variable axes, no walking the chain for a bold elsewhere.
- No change to `chainFaceNames`, the render path, `CanvasFontChainEntry`'s key set, or
  `engine-protocol.ts`'s `hasExactKeys` list — variants are already projected.
- **No prune-on-save and no document-wide asset sweep** — refused by **D-16.R.46**
  (`epic-16-decision-log.md:2244`), resting on AD-15 and **D-5.13.3**. *(An earlier draft of this
  spec cited D-16.5 here; that citation was false — D-16.5 is about refusing variable-only families
  and browser-side instancing.)*
- No second face for a family whose chain entry is a **face** entry — those carry FontSet face
  names, and `shippedFamilyEntry` already declares them at declare time.
- No retry *control* in the properties panel. The remedy is the existing re-pick path; nothing asked
  for new UI here.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| First bold | chain `Sarabun` = `[{asset: REG}]`; store holds a face with `style: 'Bold'` | one unit: `embedFontCut` writes asset `BOLD` with its own licence record and sets `fonts.Sarabun[0].bold`, then `bold:true` commits — **one revision, one undo entry** | N/A |
| Cut already declared | `fonts.Sarabun[0].bold` already names those bytes | designer sends the property command alone; engine's idempotent no-op is the backstop, not the plan | N/A |
| Held, not yet embedded | store holds `'Bold'`; chain declares no `bold` | **no sentence** | N/A |
| Upstream publishes none | census `published` lacks `'Bold'` | *"No bold face in this family — the engine paints the regular face and warns."* verbatim; engine still warns | N/A |
| Fetch refused, transient | census `refused` has `'Bold'`, `permanence: 'transient'` | its own sentence: the family has a bold, this machine could not fetch it, and a re-pick will try again | family stays `addable`; re-pick re-attempts (`fetchWebFamily(…, held)`) |
| Fetch refused, permanent | census `refused` has `'Bold'`, `permanence: 'permanent'` | its own sentence naming that this engine cannot use that bold; **no retry offered** | `familyIsComplete` already treats it as settled |
| No census row | face records exist, census row absent (v1 store, a failed `putCensus`, or `installFamily`'s partial-write path) | the **fourth sentence** (D-owner-4): the designer has not checked, so it claims nothing about upstream either way, and names the re-pick as the way to find out | re-pick records a census and moves the family into one of the other three states |
| Cut bytes equal the base | held `'Bold'` hashes to the entry's own `AssetKey` | designer never sends it; reads as genuinely absent (a bold identical to the regular is no bold) | engine's self-reference refusal is the backstop |
| Member refused | the face carries `fvar`, a contradicted licence, or a blank record field | the **whole unit** is refused, so `bold` does not commit; document unchanged; the refusal surfaces through `applyProperties` as a `propertyError` anchored on the intent's fields, so it renders **beside the toggle that was pressed**. *(Amended at review: this row said "via `refuseFontChain`", which was stale prose — that path serves the designer-side pre-checks only.)* | member error propagates verbatim with its own located path |
| Target is a face entry | chain entry is a bare face name | refused — a face entry's variants are FontSet face names, never asset keys (AD-8) | located at the entry |

</frozen-after-approval>

## Forks settled during planning

Recorded here rather than asked, per the coordinator's instruction to settle what investigation can settle.

1. **Command shape — `embedFontCut`, a second kind, 13 fields.** The earlier framing was a trade-off
   only because fusing was the sole route to one undo entry. Units removed that pressure, so nothing
   argues for a mode switch. Measured against the tree: `embedFontFamily` **refuses a chain name that
   already exists** (`:4547`) — the exact opposite precondition — and `componentFields` is an exact
   count (`componentFields(raw, 12)` at `:4452`), so no optional argument is possible. No command in
   the closed switch mutates an entry in place. Widening would move four 12-field literals and leave
   one handler with two disjoint preconditions. Fields: `kind`, `version`, `name`, `index`, `cut`,
   the six record fields, `mediaType`, `data`. `index` (not a base-key field) because
   `addFontChainEntry`/`move`/`remove` already target by index; resolving the index yields the base
   key, so the self-reference check costs no extra field.

2. **`assetKeyReferenced` — split it; do NOT widen it in place.** My earlier note said "make it walk
   `EmbeddedAssetKeys()`". Re-measured, that is **wrong and would break the base pick.** The function
   has three production callers asking *two different questions*:
   - **Safety** (`:1376` image-asset swap, `:4412` `dropUnnamedFontAssets`) — "may these bytes be
     deleted?". This arm **must** become variant-aware. Its own comment records DW-80, the identical
     bug one level up: the walk answered false for every font asset, and shipping the orphan drop
     over it *"would have deleted a face a live chain was still drawing with"*. This story makes
     variant keys reachable, so it re-opens exactly that hole one level down.
   - **Dedupe** (`:4543`, inside `embedFontFamily`) — "is this pick already in the document?".
     Widening this one would make a base pick whose Regular bytes happen to equal another family's
     declared bold **silently create no chain**. It must keep the narrow body and the existing
     red-proof, under its own name.
   `dropUnnamedFontAssets` must also consider a removed entry's variant keys, not only its base.

3. **TypeScript unit assembly — one seam, not 45.** `commandBytes(kind, fields)` fuses "build the
   object fragment" with "encode"; `encode` is private, and all 45 builders across 11 files return
   `ArrayBuffer`. Export `commandFragment(kind, fields): string` from `command-json.ts`, redefine
   `commandBytes` over it, and add `commandUnitBytes(members: ReadonlyArray<string>)`. `jsonArray`
   already takes fragments and `chainEntry` is already a fragment-returning function, so no new
   nesting machinery is needed. Only the two builders this story puts in a unit need a fragment
   twin, and each is built from **one shared field list with two terminals** so the pair cannot
   drift. Everything lives in `command-json.ts`, the soleness authority, so the two hardcoded factory
   lists (`command-json-soleness.test.ts:98`, `:119`) do not move and no new `*-command.ts` is added.

4. **Local (committed) tier keeps today's single sentence.** The `local` arm of `FamilySource` has
   no census, and it needs none: a committed family's faces are **upstream files committed to this
   repository byte for byte** — each `NOTICE.md` records *"copied unmodified, no derivation"* — and
   nothing in the designer fetches them. So the set of cuts a committed family has is fixed in the
   repository, the catalogue is its own census, and absence there is genuine absence. Story 3 keeps
   this true when it commits the cuts, because they arrive by the same route the Regulars did.
   *(An earlier draft said these faces were "instanced at build time". That is false and was
   corrected in SPEC.md before this re-plan: `tools/fontgen/instance_faces.py` drives a hardcoded
   list of seven **engine** faces, all Noto, writing to `folio-go/fonts/`, and has never produced a
   catalogue face. The conclusion above did not depend on the false claim, but the claim was
   propagating.)*

5. **Several families in one selection.** `selectionMissingCut` fires only when every selected
   component is missing the same cut, but they may be different families. One unit carries one
   `embedFontCut` member per distinct family needing that cut, plus the single property command —
   within the 1..64 bound.

## Code Map

- `folio-go/component_commands.go` — `embedFontFamily` `:4451` (`componentFields(raw, 12)` `:4452`,
  chain-name refusal `:4547`, dedupe `:4543`); reuse unchanged: `embeddedFontRecord` `:4600`,
  `embeddedFaceBytes` `:4644`, `fontChainName` `:4150`, `boundedChainFaceName` `:4966`. Gate order to
  copy: `RefuseVariableFace` `:4505`, then `RefuseContradictedLicence` `:4534`, then the asset write.
  Stale comment to rewrite: `:4583-4585`. `assetKeyReferenced` `:1419-1444` (doc comment `:1382`
  — read the DW-80 paragraph before touching it). `dropUnnamedFontAssets` `:4407`.
  `maxCanvasFontChainEntries = 64` `:4057`.
- `folio-go/component_commands.go` — the unit door, for reference only, **do not change**:
  `unitCommandKind = "applyCommands"` `:392`, handler dispatch `:325`, `carriedCommands` `:441`,
  `componentFields(raw, 3)` `:482`, bounds `:376-379`, nesting refusal `:496`, member re-entry `:520`.
  ⚠ `command_unit_test.go:399` requires **exactly one** non-test file to contain `"commands"`.
- `folio-go/internal/template/model.go` — read side, byte-identical since `431e288`, **do not
  change**: `FontChainEntry` `:197`, `fontChainVariants` `:246`, `Embedded()` `:269`,
  `SerialisesAsObject()` `:283`, `Variant()` `:304`, `EmbeddedAssetKeys()` `:321`.
- `folio-go/internal/template/parse.go` — the rules the door mirrors: namespace `:636-656`,
  self-reference `:628-631`, `requireEmbeddedFaceLicence` `:651`. Do not change.
- `folio-go/page_setup.go:201,226-231` + `internal/designer/page_setup.go:606-615` — variants already
  projected and bounded at 512. Do not change.
- `folio-designer/src/command-json.ts` — `jsonObject`/`jsonArray`/`jsonString` (fragments are plain
  `string`), private `encode`, and the single terminal `commandBytes(kind, fields)`. This is the
  seam.
- `folio-designer/src/font-chain-command.ts` — `FontChainEntryAsk` `:63`, `variantKeys` `:69`,
  `chainEntry` `:74-82` (the existing fragment-returning function; absent cut = **absent key**),
  `embedFontFamilyCommand` `:127-147`.
- `folio-designer/src/component-property-command.ts` — `updateComponentPropertiesCommand` (returns
  `ArrayBuffer` via `commandBytes`). ⚠ the soleness test forbids `String(` in factory files.
- `folio-designer/src/font-store.ts` — `FaceCutPermanence` `:170`, `FamilyCutRefusal` `:173`,
  `FamilyCensus` `:201` (`published` / `refused` / `recordedAt`; **no `held`**), `censusIsComplete`
  `:230`, `StoredFace` `:244`, `listCensus` `:295`, `putCensus` `:297`.
- `folio-designer/src/font-index.ts` — `FamilySource` `:88-95` (`stored` arm carries
  `faces` + optional `census`), `regularCutOf` `:114`, `familyIsInstalled` `:416`,
  `familyIsComplete` `:460`.
- `folio-designer/src/font-source.ts` — `faceCuts = ['Regular','Bold','Italic','Bold Italic']`
  `:204`, `RefusedCut` `:324`, `FetchOutcome` `:346`, permanence classification in `fetchCut` `:636`,
  the retry sentence `stalledRefusal` `:478`.
- `folio-designer/src/App.tsx` — `storedFaces` `:677`, `familyCensuses` `:684`, `carriedFaceKeys`
  `:900` (already flat-maps variant keys — no prefetch change), `refuseFontChain` `:2553`,
  `dispatchEmbed` `:2588`, `installFamily` `:2727`, `embedInstalledFamily` `:2943`,
  `ComponentProperties` decl `:4596` / call `:4141` (**already receives `storedFaces` AND
  `familyCensuses`**), B/I toggles + `absentCuts` `:4659`, `selectionMissingCut` calls `:4650-4651`,
  `CUT_NAMES`/`StyleCut` `:5368-5369`, `chainDeclaresCut` `:5399`, `missingCutFor` `:5425`,
  `selectionMissingCut` `:5440`, `cutAbsenceSentence` `:5462`, `cutAbsenceId` `:5468`.
- ⚠ **Two cut vocabularies.** The store/source layer uses RIBBI subfamily strings (`'Bold'`,
  `'Bold Italic'`); the chain and the panel use `'bold'|'italic'|'boldItalic'` (`StyleCut`). The
  bridge between them is this story's, and it belongs in one named place, not inline at each use.
- Existing proof of the read side: `folio-go/style_face_embedded_test.go:110`,
  `internal/template/font_chain_variants_test.go:515`, `canvas_font_chain_entry_test.go:255`.

## Tasks & Acceptance

**Execution:**
- [x] `folio-go/component_commands.go` -- add the `embedFontCut` kind (13 fields) as a case in the closed component switch -- attaches one variant asset key to `fonts[name][index]`, reusing `embeddedFontRecord`/`embeddedFaceBytes` and running the same three admission gates in the same order; refuses a missing chain, an out-of-range index, a face entry, a cut outside the closed set, a self-referencing key, and a cut already declared over different bytes; no-ops when already declared over the same bytes.
- [x] `folio-go/component_commands.go` -- split `assetKeyReferenced`: the safety arm walks `EmbeddedAssetKeys()`; the `embedFontFamily` dedupe call gets its own narrow predicate keeping the old body -- widening in place would make a base pick silently create no chain (see Forks settled, 2).
- [x] `folio-go/component_commands.go` -- `dropUnnamedFontAssets` considers a removed entry's variant keys, not only its base -- otherwise deleting a chain drops a face a second chain still draws with (DW-80, second edition).
- [x] `folio-go/component_commands.go:4583-4585` -- rewrite the "a pick embeds ONE face / the entry declares no cut" comment -- it will stand over code that no longer does what it says.
- [x] `folio-designer/src/command-json.ts` -- export `commandFragment(kind, fields)`, redefine `commandBytes` over it, add `commandUnitBytes(members)` -- the single seam that makes `applyCommands` reachable from TypeScript.
- [x] `folio-designer/src/font-chain-command.ts` -- add `embedFontCutCommand` and its fragment twin from one shared field list -- the wire shape later stories build on.
- [x] `folio-designer/src/component-property-command.ts` -- add a fragment twin of `updateComponentPropertiesCommand` over its existing field list -- two terminals, one list, no drift; no `String(`.
- [x] `folio-designer/src/App.tsx` -- send the cut embed and the property commit as one unit on first use of a cut, wired from the B/I toggles; skip the embed member when the projection already declares the cut or the held bytes equal the entry's base -- reuse `refuseFontChain` for every anticipated refusal.
- [x] `folio-designer/src/App.tsx` -- replace the single cut-absence state with the three of D-owner-3, reading `familyCensuses` (already a prop) and deriving held-ness from face records -- add the RIBBI↔`StyleCut` bridge as one named function.
- [x] `folio-go/component_commands_test.go`, `folio-go/command_unit_test.go`, `folio-go/internal/wasm/` -- cover every Go-side matrix row, the two predicates' split (red-proved by deleting each arm), and one revision/one undo for the real pair.
- [x] `folio-designer/src/*.test.ts(x)` -- pin the new builders' exact arity, the unit's assembled bytes, and all three warning states including the no-census row.

**Acceptance Criteria:**
- Given an embedded `Sarabun` entry and a held `'Bold'` face, when the author presses **B** for the first time, then the document gains exactly one asset carrying its own six-field licence record, `fonts.Sarabun[0].bold` names it, the text paints bold with no diagnostic, and **one undo restores the document byte-for-byte**.
- Given a document declaring no variant anywhere, when it is round-tripped, then its bytes and its version stamp are identical to before this story.
- Given a family whose census says upstream publishes no bold, when the panel renders, then the existing sentence appears with its words unchanged.
- Given a family whose census records a *transient* bold refusal, when the panel renders, then a different sentence appears saying the bold exists and could not be fetched, and the family is still offered in the Add font dialog so a re-pick retries it.
- Given a family whose census records a *permanent* bold refusal, when the panel renders, then a sentence appears that names that and offers no retry.
- Given a cut face that fails any admission gate, when first use is attempted, then the whole unit is refused, `bold` is not committed, the document is unchanged, and the refusal is located.
- Given a chain deleted whose entry's variant asset is still named by a second chain, when the delete is applied, then that asset is retained.

### Patch pass — 2026-09-20, applied on top of `414c05f`

All 13 `patch` entries applied and mutation-red-proved, each reddening only its intended tests. The
two `intent_gap` entries (#3, #4) were ruled by the owner and the frozen block amended in place
(D-owner-4 and the corrected "Member refused" row); no loopback was taken and no code was reverted,
because the gaps touched one ternary and one row while the implementation was otherwise sound —
`review_loop_iteration` therefore stays at 0.

Notable outcomes beyond the literal fixes:

- **#1 was the story's headline capability, broken.** `cutEmbedPlan` now asks `base.entry[cut]`. The
  fixture comment that called a single-entry chain "the shape `embedFontFamily` writes" was corrected
  — it is false, a pick appends a script-fallback tail — and `kanitRealisticChain` (base + a
  `Noto Sans Thai` declaring `bold` + `Noto Sans SC`) now drives a press, so the entry-vs-chain
  distinction is measured rather than assumed. `chainDeclaresCut` gained a paragraph saying its two
  callers ask different questions: the warning is chain-wide (per-rune resolution, Q2), the embed
  plan is entry-level.
- **⚠ The committed state at `414c05f` carried two raw NUL bytes in `App.tsx`** — one inside a
  template literal, one inside a line comment, from a `\u0000` that the edit tool decoded literally.
  Measured per revision: `9d33fbe` 0, `414c05f` 2, working tree 0. Inert in JavaScript, which is
  exactly why nothing caught it: no linter, no typecheck and no test reads source bytes for C0
  controls. A repo-wide C0 assertion in the existing source sweep would close the class; it was NOT
  added here because a repo-wide guard is outside this story's fence.
- **Two defensive branches are unreachable, not merely untested**, and were kept with the
  unreachability written in at each site: `store === undefined` in `commitPropertiesEmbeddingCuts`
  (both `setStoredFaces` call sites sit behind a truthy store, and a plan requires non-empty
  `storedFaces`, so reaching it needs a store that opened and then vanished) and its `catch`
  (`font-store.ts`'s `transact` wraps its whole body and returns an outcome, so `store.get` does not
  reject). Deleting either restores a real hazard — a misdirecting sentence and a dead toggle — so
  both stand. The property the catch protects IS measured, by *"leaves the toggle usable after a
  refusal"*.
- **Rejected findings stay rejected**; the four noted as deferrable were not written to
  `deferred-work.md`, because they are observations about guards and polish rather than work items,
  and each is recorded above with its reasoning.


## Implementation Notes

**Built 2026-09-20 on `9d33fbe`.** Every settled fork was implemented as planned; the three
decisions below were taken during implementation and are recorded rather than asked.

**Amended at review (`414c05f` → follow-up).** Eight defects were fixed; the substantive ones:
- **`cutEmbedPlan` asked the CHAIN and had to ask the TARGET ENTRY.** Since Story 11.4 a pick
  appends `proposedFallbackTail`, and `Noto Sans Thai` declares a `bold` — so for every family whose
  scripts exclude Thai the chain always declared a bold, no plan was ever built, and pressing **B**
  embedded nothing and said nothing while **I** still worked. `chainDeclaresCut` stays chain-wide
  for the WARNING (the engine resolves per rune, Q2's all-entries rule); the plan is entry-level.
  Every fixture here had been a one-entry chain, on which the two rules agree.
- **The fourth absence state (D-owner-4) was added.** `census === undefined` returned `'unfetched'`,
  rendering *"This family HAS a bold face"* from no census at all. It now returns `'unchecked'`,
  whose sentence claims nothing about upstream in either direction and names the re-pick.
- **The combined cut kept its stated exit in the new states**, which an early return had dropped,
  and both new sentences gained an article helper (`an italic`, not `a italic`).
- **`applyCommandUnit` gained a whole-payload bound** (`maxUnitPayloadBytes`).
  `maxComponentAssetBytes` is a PER-MEMBER derivation, so several faces in one unit could clear
  every member bound and still fail at transport with no located diagnostic.
- **`commitPropertiesEmbeddingCuts` gained a `catch`** — a rejection escaping into `onCommit` left
  `BooleanProperty`'s `pendingRef` set and the toggle dead for the session — and a store-unavailable
  sentence of its own, since telling that author to re-pick prescribes a retry that must fail.
- **`unitCommandKind` is now tied to `command-json.ts`'s `'applyCommands'` literal** by
  `command_json_authority_wire_test.go`; changing the Go constant's value previously left both
  suites green while every unit fell through to "unknown component command".

- **The panel's unavailable state is the ABSENCE of an embed plan, not a second predicate.**
  `cutEmbedPlan` is the one function the dispatch builds its unit members from AND the one the
  toggles' unavailable state is derived from, so the sentence an author reads can never disagree
  with what the next keypress actually does. A separate held-ness predicate would have let the panel
  say *"No bold face in this family"* about a bold the press was about to embed — which is exactly
  the falsehood CAP-2 names.
- **A selection whose families disagree about the REASON states no sentence.** `selectionMissingCut`
  already required every selected component to be missing the SAME cut; it now also requires them to
  agree on the absence state. That is the file's own stated rule ("a selection missing two DIFFERENT
  cuts has no one true sentence to state, so it states none") applied to the axis the census opened,
  and by the Design Notes' test it only ever ADDS a way to reach "no warning".
- **The family is resolved off the embedded entry's own `family` record, not the chain name.** The
  engine already projects `CanvasFontChainEntry.Family` from the asset's `font` record, so the store
  and the census are keyed by what the DOCUMENT says the face is. The chain name is the fallback for
  an entry whose record carries none.

**Go.** `embedFontCut` is a case in the closed component switch, applied through
`applyFontChainCommand` like every other chain command. Refusals about the target are located at
`fontChainEntryPath(name, index)` — a new helper that truncates the NAME rather than the index, so a
long chain name cannot silently turn an entry-level path into a chain-level one. `commandFontChainCuts`
gained a named row type (`commandFontChainCut`) so the resolver can return one without a second
hand-copied spelling of the table's shape. `assetKeyReferenced` (safety) now walks
`EmbeddedAssetKeys()`; `embeddedBaseKeyReferenced` (dedupe) keeps the old body under its own name;
`imageAssetKeyReferenced` is the arm both share, factored out so the three-walk warning has one home.

**TypeScript.** `commandFragment` is the split seam; `commandBytes` is redefined over it and emits
identical bytes (asserted). `commandUnitBytes` is the only spelling of `applyCommands` on this side.
`applyProperties` gained an optional `payload` override so the unit and the bare command share one
error path, one snapshot installation and one refusal anchor.

**Red-proofs recorded (all 2026-09-20, clean tree):**
- Narrowing `assetKeyReferenced` back to `entry.Embedded() && entry.AssetKey == key` reds
  `TestACutASecondChainStillNamesIsRetained` and `TestTheTwoAssetPredicatesDisagreeExactlyWhereTheyShould`,
  and nothing else.
- Pointing `embedFontFamily`'s dedupe at `assetKeyReferenced` reds
  `TestThePickDedupeDoesNotSeeAnotherChainsDeclaredCut`, and nothing else.
- Restoring `dropUnnamedFontAssets`' base-only body reds `TestRemovingAnEntryDropsTheCutItDeclaredToo`,
  leaving the base half green — which is what says the two arms are measured apart.
- Making `cutEmbedPlan` always answer `undefined` reds five `App.font-store.test.tsx` cases; making
  `cutAbsenceState` always answer `'unpublished'` reds the three census cases and no others.

**Not built, and named rather than left implicit:** the committed (`local`) tier still embeds its
Regular alone and keeps today's single sentence, per settled fork 4 — it writes nothing to the
machine store, so `storedFaces` holds nothing for it and both the embed plan and the census lookup
correctly find nothing. Story 3 is what gives that tier its cuts.

**⚠ ONE UNPLANNED RED, FOUND AT THE GATE AND NOT BY THE IMPLEMENTATION PASS.** The TypeScript seam
split reddened `TestCommandJsonAuthorityAndTheEnginesRefusalLandTogether`
(`folio-go/command_json_authority_wire_test.go`), a Go test that reads `command-json.ts` as SOURCE
TEXT and requires the command envelope — `'kind'`, `jsonString(kind)`, `'version'`, `jsonNumber(1)` —
to be spelled inside `commandBytes`. Splitting `commandFragment` out moved that spelling one function
along, so the extraction stopped seeing it. **The rule was still true; only the extraction was
stale**, and the test's own failure text says so: *"if the authority was restructured, re-derive this
extraction rather than deleting the check"*. So it was re-derived onto `commandFragment` — and a
SECOND assertion was added, because the split opened a hole the old guard could not see: with the
envelope checked in one function, `commandBytes` could be rewritten to build its own and every
existing assertion would stay green. It is now required to be defined over `commandFragment`.
**Red-proved**: giving `commandBytes` its own inline envelope reds the new assertion — a mutation the
old guard passed. The guard is strictly stronger than it was, and nothing was relaxed to make it
green.

## Spec Change Log

- **2026-09-20 — re-planned after stories 1 (`9506503`) and 6 (`e08f558`) landed.** Open Question 1
  (command shape) superseded and settled as `embedFontCut`; Open Question 2 already corrected in
  place by story 6 (D-6.5) and now recorded as D-owner-1; Open Question 3 settled by D-owner-2. The
  three-state warning (D-owner-3) is new scope from the owner, sourced from story 1's census. The
  false `D-16.5` citation was removed from two further sites in this file (Intent, Boundaries) that
  story 6's correction did not reach. **The `assetKeyReferenced` task was corrected**: the previous
  draft's "walk `EmbeddedAssetKeys()`" would have broken the base pick. KEEP: the byte-neutrality
  argument, the "sentence keeps its words, changes its subject" rule, and the forced embed-before-
  property ordering.
- **2026-09-20, at review — two owner amendments to the frozen block, and a unit-level bound.**
  Triggered by triage findings #3 and #4. **(a)** The three-state warning model assumed a census is
  always present; three ordinary paths reach the panel without one, and the built behaviour asserted
  *"This family has a bold face"* from no evidence — false for the 947 of 1,274 offered families that
  publish a Regular and nothing else. Owner ruled a **fourth state** (D-owner-4); the matrix row was
  rewritten and D-owner-3's reservation preserved exactly. Known-bad state avoided: a sentence that
  claims something about upstream the designer has not measured. **(b)** The "Member refused" row
  named `refuseFontChain`, which is stale prose — engine refusals surface through `applyProperties`
  and render beside the pressed toggle. Row corrected to describe what happens. **(c)** Finding #4:
  `maxComponentAssetBytes` is a per-MEMBER derivation, so a multi-face unit can exceed the transport
  bound and fail without a located diagnostic; owner ruled a **unit-level payload check in
  `applyCommandUnit`** with its own constant, keeping the bound single-authority in Go.
  **KEEP across any re-derivation:** the `assetKeyReferenced` split and its two red-proofs; the
  `commandFragment` seam and the `commandBytes`-defined-over-it tie; the one-field-list-two-terminals
  shape for both fragment twins; `fontChainEntryPath` truncating the name and never the index; and
  every sentence whose words D-owner-3 fixes.
- **2026-09-20, at approval — false "instanced at build time" claim removed from settled fork 4.**
  The committed tier's faces are upstream files copied byte for byte, not fontgen output; fontgen
  drives seven engine faces only. The fork's conclusion was independent of the claim and stands.

## Review Triage Log

Four layers ran at `414c05f`: blind-hunter (BH), edge-case-hunter (EC), verification-gap (VG), and a
targeted hazard layer (TG) aimed at the five surfaces the coordinator named plus a repo-wide sweep
for guards invisible to the suite that would normally catch them. Verdicts are mine, rendered after
checking each claim at its cited location.

| # | Finding (source) | Verdict | Evidence | Route |
|---|---|---|---|---|
| 1 | `cutEmbedPlan` asks `chainDeclaresCut` (chain-wide) while the plan targets ONE entry, so a family whose fallback tail declares the cut never embeds (BH, TG — TG reproduced it with a running probe) | **high** | Verified myself. `App.tsx:5560`. `proposedFallbackTail` (`:385`) appends `shippedFamilyEntry` rows and `shipped-face-cuts.ts:79` gives `Noto Sans Thai` a `bold`. A latin-only family (`scripts:['latin']`, most of the catalogue) gets tail `[Noto Sans Thai, Noto Sans SC]`, so `chainDeclaresCut(...,'bold')` is true and **no plan is ever built**. CAP-1's headline capability does not fire. Italic still works, so the two toggles disagree on one family. Matrix row 2 already says the question is the ENTRY's (`fonts.Sarabun[0].bold`) | patch |
| 2 | No test fixture has a fallback tail; `kanitChain()` is single-entry, which is the one shape that cannot see #1 (BH, TG) | **high** | Same root cause as #1 — grouped with it | patch (with #1) |
| 3 | `cutAbsenceState`'s `census === undefined` arm returns `'unfetched'` when face records exist, rendering *"This family **has** a bold face, but it is not on this machine"* — a positive claim about upstream from no census (TG with a running probe; EC filed the mirror claim) | **high** | Verified reachable **three ways**: v1 store records (the v1→v2 migration is additive, no census rows); a `putCensus` failure after faces land (`App.tsx:3188-3199`); and `installFamily`'s partial-write path (`App.tsx:2892-2901`), which refuses and returns **before** `recordFamilyCensus`. `font-store.ts:225` records that 947 of 1,274 offered families publish a Regular and nothing else, so the sentence is false for most of the population. **The behaviour matches the FROZEN matrix row**, so the root cause is inside `<frozen-after-approval>` | **intent_gap** |
| 4 | A unit can carry several faces, but `maxComponentAssetBytes` is derived per-command (TG) | **medium** | `component_commands.go:1260-1278` derives the bound so "a file Go is willing to accept can always actually arrive". `firstUseCutPlans` emits one member per `(chain,cut)`; the summed base64 can exceed `engineProtocolMaxPayloadBytes` and be rejected at transport instead of with a located diagnostic. The fix is a NEW bound and the spec does not say which side holds it | **intent_gap** |
| 5 | `fontChainEntryPath`'s truncation rule is unasserted; all five assertions compare the production function to itself (VG, TG, BH) | **medium** | Both layers demonstrated it: rewriting the helper to truncate the index away leaves every assertion green. TG probed the behaviour and it is CORRECT (bound 256, `[n]` suffix always survives, rune-safe) — the assertions are what is vacuous. The D-11.2.8 shape this repo rejects elsewhere | patch |
| 6 | The RIBBI bridge's `'Bold Italic'` spelling is never exercised (VG) | **medium** | Pre-verified. Mis-spelling `boldItalic: 'BoldItalic'` leaves every test green while the third cut silently stops embedding and the panel states a falsehood about a held face | patch |
| 7 | No multi-component / multi-family test; the `(chain,cut)` dedupe key and the new "reasons must agree" rule are both inert (VG ×2, BH) | **medium** | Pre-verified with demonstrations: collapsing the key to `plan.cut`, or deleting `&& one.absence === first.absence`, leaves the suite green. Settled fork 5 is unmeasured | patch |
| 8 | `cutAbsenceState`'s no-census-AND-no-records branch is untested (VG) | **medium** | Pre-verified. Blocked on #3 — the expected value is what #3 asks the owner | patch (after #3) |
| 9 | `boldItalic` loses its stated exit in the two new states (BH, EC, VG-note) | **medium** | Verified: `cutAbsenceSentence` returns early for `unfetched`/`unusable` before the `boldItalic` branch, so the combined cut is never told that turning off either control resolves it. The function's own docstring makes the stated exit load-bearing (*"a state with no stated exit is the grey-out DESIGN.md forbids"*) | patch |
| 10 | `it('reads a bold whose bytes ARE the regular as genuine absence')` asserts `NO_BOLD`, which is also the pre-listing initial state (BH) | **medium** | Verified: it settles on the store rather than on any panel change, so it passes over a panel that never consults the store. Its neighbours deliberately carry a second-cut positive control; this one does not | patch |
| 11 | No designer test covers an ENGINE-refused unit; only the store-miss path is covered (BH, EC) | **medium** | Verified. TG separately confirmed the behaviour is right — an engine refusal anchors on `intentFields` and renders under the pressed toggle | patch |
| 12 | A store that will not open is reported as a face that vanished (BH) | **medium** | Verified: `store === undefined` shares a branch with a dropped record at `App.tsx:3105`, prescribing a re-pick that fails for the same reason | patch |
| 13 | `commitPropertiesEmbeddingCuts` has `try/finally` but no `catch`; a rejecting store read leaves `BooleanProperty`'s `pendingRef` set (EC) | **medium** | Verified by reading both: `commit` sets `pendingRef.current = true`, awaits, clears — a throw skips the clear and the toggle is dead for the session | patch |
| 14 | The justification comment at `App.tsx:5594` is false — a space CAN occur in a chain name (BH, VG) | **low** | Verified: `Noto Sans Thai` is a chain name throughout. VG established the key is collision-free for a different reason (no cut is a space-separated tail of another), so the code is right and the stated reason is not — in a file where these comments ARE the record | patch |
| 15 | Both new sentences read *"a italic face"* (TG, reproduced) | **low** | Verified: `CUT_NAMES.italic === 'italic'` and the template is `a ${CUT_NAMES[cut]}`. Ordinary to meet, and the fix is a direct correction | patch |
| 16 | `firstUseCutPlans` silently returns `[]` for the `PropertyIntents` array form (BH) | **low** | Verified. Unreachable today — `BooleanProperty` sends one intent — but the doc comment explains only the toggle-off exclusion | patch (comment) |
| 17 | The frozen matrix says the member refusal surfaces "via `refuseFontChain`"; engine refusals actually surface through `applyProperties`/`propertyError` (BH, EC) | **low** | Verified, and the behaviour is BETTER than the row says — TG confirmed the refusal lands under the pressed toggle. The row's mechanism name is stale prose. **Rejected on the rule that no finding's fix may be to edit this build's spec**; raised to the owner in the report instead | rejected |
| 18 | Designer-side pre-check refusals land under the Font family control, not the toggle (TG) | **low** | Verified: `refuseFontChain` sets `control.action:'embed'`, rendered inside `FontFamilyProperty`. Misplaced but not false, and both refusals concern the font. The fix adds a new `FontChainControl.action` member — public surface for a cosmetic gain | rejected (low) |
| 19 | `storedFaces.find` picks an arbitrary vintage when two coexist; no tie-break (BH) | **low** | D-owner-2 already rules that a mismatched vintage is embedded anyway, so "which one" is settled by decision. The residual — landing on a base-equal duplicate and reporting the cut held — needs two vintages AND a base-equal collision. Fix is a new filter, more than a direct correction | rejected (low), recorded as deferred |
| 20 | `embedFontCut` admits a record `style` that contradicts the `cut` (BH) | **low** | Verified: the door never compares them. Unreachable from the product — the designer derives both from one `StoredFace`. `style` is display metadata, not admission. Fix adds a check guarding state never demonstrated | rejected (low) |
| 21 | `missingCutFor` suppresses the sentence whenever a plan is POSSIBLE, so an already-bold component whose bold was never embedded shows no sentence while the engine warns (EC) | **low** | Verified the state is real. But no FALSE statement is made — the panel is silent, not wrong — and the honest fix is a fourth message, well beyond a direct correction | rejected (low), recorded as deferred |
| 22 | Cut lands on a non-painting entry when leading entries are face entries (EC) | **false** | `embeddedChainBase` returns the first EMBEDDED entry; the engine resolves per rune across entries in order, so for the runes that entry covers its cut is the one used. `embedFontFamily` writes `[asset, ...tail]`, so index 0 anyway for anything the designer creates | rejected |
| 23 | `TestExactlyOneFunctionReadsAUnitsMemberList` counts the raw literal `"commands"` across the WHOLE file text, comments included; this diff added ~300 lines of prose about `applyCommands` next to it (TG) | **low** | Verified: the count is exactly 1. Green, but by luck — one double-quoted mention in a comment would red a Go test from a Go comment | rejected (low), recorded as deferred |
| 24 | `unitCommandKind = "applyCommands"` (Go) and `commandFragment('applyCommands', …)` (TS) are tied by nothing (TG) | **medium** | Verified: Go tests spell `unitCommandKind`, TS tests pin the literal. Changing the Go constant's VALUE leaves both suites green while every unit the designer sends falls through to `"unknown component command"`. This diff CREATED the exposure by making `applyCommands` reachable from TypeScript for the first time | patch |
| 25 | `TestEveryDesignerCommandFactoryRoutesThroughTheAuthority` is green by one character — `component-property-command.ts` added `import type { JsonField }` on its own line, which does not match the required `import {`, and the value import on the next line still does (TG) | **low** | Verified: lines 15 and 16. Had the value import been dropped for the type-only form, a Go test would have gone red from a TypeScript edit with `npm test` green | rejected (low), recorded as deferred |

**Outcome: two intent_gap entries (#3, #4) block the rest.** Per the workflow's cascading order they
trigger a loopback, and the patch entries are held until they are resolved — #8's expected value is
literally what #3 asks. Both root causes sit inside `<frozen-after-approval>` or require a new bound
the spec does not place, so only the owner can settle them.


## Design Notes

**Byte neutrality is structural.** `writeFontChain` asks `SerialisesAsObject()`, and
`fontsRequireMajor` asks the identical predicate. A document declaring no cut emits the same bytes
and the same version stamp. No golden should move; if one does, that is a finding, not a rebaseline.

**The sentence keeps its words and changes its subject.** `chainDeclaresCut` returns `true` (= no
warning) for an unknown family or a missing chain; every new source may only ADD a way to reach
`true` or REPLACE which sentence is shown — never a new way to reach a false "this family has no
such face".

**Absence of a census is not evidence of absence upstream.** A v1 store record survives the additive
v1→v2 migration with no census row. `familyIsComplete` already treats `census === undefined` as
incomplete; the sentence follows the same reading rather than inventing a second one.

**Order is forced by the engine**, not by taste: the cut must be embedded before the property is
committed, because the engine refuses a property naming something the document has not declared.
Inside a unit, members apply in order against one document, so ordering is preserved.

## Verification

**Baseline measured at `9d33fbe`, clean tree, 2026-09-20 — nothing below may regress from it:**
- `folio-go` `go test -count=1 ./...` → **3169 pass / 2 fail / 5 skip**; the two are
  `TestCorpusMeetsP6ExerciseFloors` and its `P6g_(opaque_names)` subtest — one distinct pre-existing
  red, not a gate.
- Designer `npx oxlint` → exactly **8** `only-export-components` warnings.

**Commands** (run each separately — a conjunction drops everything after a known-failing term):
- `cd folio-go && go test -count=1 ./...` -- the two pre-existing reds only; report the pass count.
- `cd folio-go && go vet ./...` -- exit 0.
- `gofmt -l /Users/panitw/Projects/folio/folio-go` from the **repo root** -- empty.
- `cd lint && go test -count=1 ./...` -- four `ok`; `-count=1` is mandatory, a cached `ok` is not a measurement.
- `cd folio-designer && npm test` -- all passing; report file/test counts against the baseline taken before the first edit.
- `cd folio-designer && npx tsc -b --force` -- exit 0.
- `cd folio-designer && npx oxlint` -- exactly 8 `only-export-components`, no other warning.
- `cd folio-go && GOOS=js GOARCH=wasm go test -count=1 -exec="$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./wasm/cmd/engine/` -- the wasm-host boundary is invisible to `go test ./...`.
- Byte identity: no golden or fixture digest may move. Report `shasum -a 256 fixtures/*/expected.pdf` unchanged.
