---
title: 'Fetch and store a family''s whole face set'
type: 'feature'
created: '2026-09-19'
status: 'done'
baseline_commit: 'cfbe5daf8639478a28a1b22850997dee1b0e08f0'
route: 'dispatch'
review_loop_iteration: 1
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Installing a web family keeps one face. `regularFilename` (`font-source.ts:196-198`) narrows the parsed `METADATA.pb` to the weight-400 upright and throws the family's other cuts away, and `fetchWebFamily` stamps the one survivor `style: 'Regular'` (`:532`). Downstream, `font-index.ts` folds a family's stored faces back to one (`mostRecentlyFetched` `:257-260` feeding `storedByFamily` `:273-278`) — and because `fetchedAt` is day-granular, four faces installed in one session tie and fall through to the lowest key, an arbitrary cut rather than the Regular.

**Approach:** Make a pick install every cut the family publishes into the designer's local face store, and make `FamilySource` hold a family's face *set* instead of one face, so one family is offered once and each cut resolves to its own face. The collapse is replaced, not tie-broken.

## Boundaries & Constraints

**Always:**
- The upright Regular stays the base and stays required. A family with no `style: "normal"`, `weight: 400` face is still refused outright at `font-source.ts:448-453`, before any byte is kept.
- Per-face checks stay per face: the `fvar` refusal (`:513-515`) and the `.ttf`/`.otf` media-type check (`:206-211`) run over each cut's own bytes. One bad cut is refused on its own; a family whose italic is refused still installs its Regular and Bold.
- Cuts are identified from `METADATA.pb` only: bold is `weight: 700` upright, italic is `style: "italic"` at 400, bold-italic is `style: "italic"` at 700. Nothing is synthesised and no name is constructed from the family string.
- `style` on a stored record is the face's RIBBI subfamily — exactly `Regular`, `Bold`, `Italic` or `Bold Italic`. `font-store.ts:234` already requires it non-empty; this is where its value starts mattering.
- The abort-terminates-the-chain contract holds for the **base** chain (directory probe → licence → Regular). Each additional cut's fetch is independently refusable and never aborts the family.
- One pick is one row in the browser and one `onAddFamily` call, whatever the face count.

**Never:**
- No change to the document. No embed, no chain entry, no engine command, no `.folio` byte. `embedInstalledFamily` still carries the Regular alone and the panel still warns exactly as it does today. That is story 2.
- The **face** store stays content-addressed: face records keyed by the SHA-256 of the bytes, no per-family uniqueness, and no second authority on a fact a record already carries. The family census (D-7) is a **separate** store and the only family-keyed thing in the module.
- No change to the committed catalogue's data or to `build-wasm.mjs` (story 3), and none to `font-index.json` / `build-font-index.mjs` (story 5).
- Do not repair the collapse by improving `mostRecentlyFetched`'s tie-break. Delete the mechanism and replace the comment at `font-index.ts:264-272` with one that states the new model.
- No synthetic bold or oblique, no variable faces, no weight outside the four cuts.

## Decisions (owner, 2026-09-20)

- **D-1 — A cut's store write fails partway: refuse the install, leave what landed.** Already-written cuts stay; there is no delete path and none is added. The store is content-addressed, so an orphan is harmless and a retry reuses it rather than refetching. The refusal is stated at the control the author acted on, as it is today.
- **D-2 — A skipped cut installs silently, but the skip is RECORDED on the family.** No message at pick time. The record is **readable by the installed/complete predicate**, never write-only.
  - **AMENDED 2026-09-20 after review (owner, ruling 1).** D-2 originally named a **two-way** distinction — *upstream refused / fetch failed* vs *never attempted* — and had **no third state for a transient failure**. The implementation was faithful to it, so a stalled body and an offline connection were recorded exactly like a variable `fvar` or a 404, which settled the cut for ever and stranded a Bold the family really publishes. **The intent was incomplete, not the implementation.** A recorded refusal now carries whether it is **PERMANENT** (a variable `fvar`, an unreadable media type, a 404, upstream publishes no such cut) or **TRANSIENT** (a stalled body, offline, any network-shaped failure), and `recordedAt` is what a retry policy reads.
  - **The classification is derived from the failure itself, never guessed at the call site.** A failure shape that cannot be classified confidently is **TRANSIENT**: retrying something permanent costs one wasted request; stranding something transient costs a permanently missing Bold with no path back.
- **D-3 — A stored family whose cut set is short is installable again.** Picking it fetches **only the missing cuts**. `installFamily`'s refusal of a `stored` row (`App.tsx:2717`) stops being unconditional.
- **D-4 — `familyIsInstalled` means the family holds every cut it publishes.** Holding only the Regular is no longer "installed". The owner accepted that this moves incomplete families back into the installable group and reds the e2e-pinned "Install N on this machine" count: **correct that assertion, do not work around it.**
- **D-5 — The completeness predicate, which is what keeps D-2/D-3/D-4 from becoming a silent retry loop.** A family counts as **installed** when it holds every cut it publishes **OR carries a recorded PERMANENT refusal for each cut it lacks** (amended with D-2, ruling 1). A **transient** refusal does **not** settle a cut: the family reads incomplete and that cut is retried on a later pick. The plain-words acceptance test for this amendment is `font-source.ts`'s own stall sentence — *"Try the pick again if you like"* — which must be **true again for every cut, not only the base**. That is what D-2's record is for. A family publishing only a Regular is therefore complete the moment it holds that Regular and never re-offers — **measured: 947 of the 1,274 offered web families, 74.3%** (`font-index.json`, `axes == []`, `styles == ["400"]`). The remaining 327 are what D-2's record exists to terminate.
- **D-6 — Scope, settled with the answers.** `font-index.ts:119` `localByFamily` is **in** scope. Narrowing the abort-terminates-the-chain contract to the base chain is **approved**. `weightLine` (`font-browser-model.ts:377`) is **not** — this story falsifies its "no bold or italic" sentence and **story 5 is sequenced to repair it; leave the string alone**. `Buda` / `Molle` / `UnifrakturCook` — offered by the dialog, refused by the installer for having no upright 400 — stay **out** of scope; do not widen `addableFromTheWeb`.

- **D-7 — The recorded refusal lives in a NEW family-census object store in `font-store.ts` (owner, option (a)).** Keyed by family, holding what upstream publishes, what is held, and what was refused and why. Chosen because it is **one authority on one fact**, which is that module's own stated principle — a census copied onto each face record would be N copies that can disagree, the thing `font-source.ts:240`'s own comment refuses in those words. **This store's API is inherited by stories 2, 3 and 4: whoever picks those up uses it and does not invent a second one.** Obligations that came with the approval:
  - The `databaseVersion` bump (`font-store.ts:91`, currently `1`) is **additive**, in the shape `onupgradeneeded` already uses at `:296-297` with its `contains` guard. **An author's held faces must survive the upgrade** — silently wiping the store is the failure mode that ruled the per-face-field option out, and it may not reappear through the upgrade path.
  - A census must distinguish **three** things, because D-5's predicate needs all three and story 2's panel sentence will too: *upstream publishes this cut and the fetch failed* / *upstream publishes no such cut* / *never attempted*.
  - **AMENDED 2026-09-20 after review (owner, ruling 2): the census carries NO `held` field.** D-7 first said the census holds "what is held"; it does not, and that is deliberate and **must not be "restored"**. Which cuts this machine holds is read off the **face records**, every one of which already carries a `style`. A `held` list here would be a second copy of a fact the face set already states, and it would **drift**: the face store self-heals by dropping a record it cannot verify, and a `held` list would go on claiming a cut whose bytes had just been dropped. This is D-7's own *one authority on one fact* applied to itself.
  - The census is **the** authority on what a family publishes. `generated/font-index.ts`'s `styles` must not become a second one — that carry-through belongs to story 5 and stays out of this story.

- **D-8 — A family whose faces are held is USABLE, whatever its census says (orchestrator, direction 3).** D-4 moved incomplete families back into the *installable* group; it did **not** put them out of reach. A stored family with no census — anything installed before this story — must not vanish from **AVAILABLE LOCALLY**, because offline that makes a font already on the machine unusable, which is a strictly worse outcome than the one the owner accepted. **Completeness governs whether a family is re-offered for install. It does not govern whether the faces on this machine can be used.**

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Full family | `METADATA.pb` publishes all four cuts | Four records in the store, one per cut, each with its own `style`, `source` and licence record | N/A |
| Partial family | Family publishes Regular + Bold only | Two records; no italic record, no placeholder | N/A |
| No Regular | No `normal`/400 face | Whole family refused before any fetch of bytes | Existing refusal, reworded off "single face" |
| Cut is variable | Bold carries `fvar` | Regular installs; Bold skipped **and recorded refused** | Per-cut refusal, family not poisoned |
| Cut is `.woff2` | Bold filename has no media type | Regular installs; Bold skipped **and recorded refused** | Per-cut refusal |
| Regular stalls | Abort on the Regular's body | Family refused; nothing kept | Existing stall wording |
| Bold stalls | Abort on the Bold's body | Regular installs; Bold skipped **and recorded refused** | Per-cut, chain not terminated |
| Partial store write | Regular written, Bold's write hits quota | Install **refused**; the Regular stays written (D-1) | Stated at the control; no delete, no rollback |
| Re-pick a short family | Store holds Regular only, upstream publishes Bold | Fetches **only** the Bold; the Regular is not refetched (D-3) | Per-cut as above |
| Complete family re-offered | Holds all cuts it publishes, or a refusal for each it lacks | Reads **installed**; not offered for install again (D-5) | N/A |
| Family with no census | Installed before this change; holds a Regular, census absent | Every cut it lacks reads **never attempted**, so it reads incomplete and is offered for install again (D-3) | N/A |
| Store upgrade | A v1 store holding faces is opened by this build | Upgrades to v2, census store created, **every existing face record still readable** | Upgrade is additive; no wipe |
| Transient cut failure | Bold's body stalls, or the machine is offline | Regular installs; Bold recorded **transient**; family reads **incomplete** and the Bold is retried on a later pick | "Try the pick again" is true for it |
| Permanent cut failure | Bold carries `fvar`, or 404s | Regular installs; Bold recorded **permanent**; family reads **complete** and never re-offers | D-5's loop stays closed |
| Held but census-less, offline | Family installed before this story; no network | Still listed in **AVAILABLE LOCALLY** and still usable (D-8) | Never unusable for want of a census |
| Local-tier family | Catalogue family holding its one committed face | Reads **installed** — the catalogue is the authority on what it publishes, and it publishes one cut today | No census needed for `local` |
| Store holds four | Four faces of one family listed | Browser offers that family **once**; each cut resolves to the record whose `style` matches, stable across reloads and same-day fetches | N/A |

</frozen-after-approval>

## Code Map

- `folio-designer/src/font-source.ts` -- `FamilyMetadata.faces` (`:116-121`, parsed `:138-187`) already carries every static face with weight and style; **reuse it**. `regularFilename` (`:196-198`) is the narrowing to generalise into a per-cut picker. Base refusal `:448-453` **keep**. `mediaTypeOf` `:206-211`, `faceIsVariable` guard `:513-523`, `faceCopyright` — run **per cut**. `webFaceSource` `:240` and `style: 'Regular'` `:532` both become per-face. `fetchWebFamily` `:403` is the entry point; `timedFetcher` `:343` arms a **fresh** timeout per request, so extra cuts do not share a deadline.
- `folio-designer/src/font-index.ts` -- `FamilySource` `:77-83` (three tiers, each single-face) is the type to reshape. `mostRecentlyFetched` `:257-260` and the `storedByFamily` fill `:273-278` are the collapse to **delete**; comment `:264-272` to **rewrite**. `localByFamily` `:119` is the same last-wins defect on the local arm — reshape it now so story 3 only supplies data. `addableFromTheWeb` `:149` **do not touch**. `familyIsInstalled` and `familySourceNote` read the union.
- `folio-designer/src/font-store.ts` -- **now in scope for the census only (D-7).** `databaseName` `:90`, `databaseVersion` `:91` (currently `1` — bump), `faceStoreName` `:93`, `byteStoreName` `:109`; `openFontStore` `:286` and its `onupgradeneeded` `:294-298`, whose `contains` guard is the additive shape to copy. `StoredFace` `:122-138` and `soundFace` `:234` (validates a fixed field list; already requires non-empty `style`) — **leave the face record shape alone**. ⚠ `transact` `:325` takes its store list at `:328`: a transaction naming the census store must name it there. ⚠ `work` MUST issue every request synchronously before awaiting (the rule stated at `:310-324`) — a census write placed after an `await` lands on an inactive transaction and is silently lost.
- `folio-designer/src/App.tsx` -- `installFamily` `:2672` is the write path (`local` arm `:2694-2713` unchanged; store write `:2772`). `keepOnThisMachine` `:2926` writes one record — it is the call to run per cut. `ResolvedFace` `:2517`, `scriptsOfSource` `:386-389`, specimen bytes `:1073-1116`, `embedInstalledFamily` `:2815` and `addFamilyToDocument` `:2488` all read the single-face arms and must read the Regular out of the set.
- `folio-designer/src/font-browser-model.ts` -- `BrowserRow.source` `:114`, `rowTierNote` `:143-150` (exhaustive tier switch), `browserRows` `:156-159` read `.face.scripts` / `.record.scripts` / `.row.scripts`.
- `folio-designer/src/FontBrowser.tsx` -- `:38`, `:53`, `:104` carry the type only.
- `folio-designer/src/shipped-face-cuts.ts` -- `shippedFamilyCuts` `:77-83` is the **reference vocabulary** for how the four shipped families name their cuts. Read it; do not change it.
- **Tests that must be corrected, never deleted:** `font-source.test.ts:629-651` (literal abort-chain table `[0,1] [1,2] [2,3]`) and `:674-686` ("exactly three requests" on the success path) — the Kanit fixture (`:27`) publishes an italic-400, so a whole-family fetch changes both counts. `font-index.test.ts:344-366` and `:368-382` pin `mostRecentlyFetched` and its same-day tie: **replace** them with the new model's assertion (four faces of one family offered once, each cut resolving by `style`), do not drop them. `font-provenance.test.ts:155-180` `toEqual`s the exact list of `source:` emission sites in `font-source.ts`. `App.font-store.test.tsx:46-53` stubs upstream with only `Kanit-Regular.ttf`, and `:280 :618 :635 :715 :770` assert exact store listings. `FontBrowser.test.tsx:461-470 :503-519` build `FamilySource` literals and pin one `onAddFamily` per family.
- **e2e:** `folio-designer/e2e/font-browser.spec.ts:133-154` asserts `Install 2 on this machine` for two staged **families** — that count must stay family-counted, not face-counted.

## Tasks & Acceptance

**Execution:**
- [x] `folio-designer/src/font-store.ts` -- add the family-census object store per D-7: bump `databaseVersion` to `2`, create the store in `onupgradeneeded` behind the same `contains` guard, and add read/write API plus a `soundFace`-style validator for the census record -- one authority on what a family publishes, held and refused; the face record shape and the byte store are untouched.
- [x] `folio-designer/src/font-store.test.ts` -- cover the additive upgrade explicitly: a v1 store holding face records, opened by this build, upgrades to v2 and **every existing face record is still readable** -- this is the failure mode that ruled out the per-face-field option and it must be proven, not assumed.
- [x] `folio-designer/src/font-source.ts` -- generalise `regularFilename` into a picker returning the family's RIBBI cut set from `FamilyMetadata.faces`; make `fetchWebFamily` fetch, verify and describe each cut independently and return the set; make `style` and `source` per-face -- the upstream data is already parsed and only the narrowing discards it.
- [x] `folio-designer/src/font-index.ts` -- reshape `FamilySource` so every tier holds a family's face set; rewrite `familyIsInstalled` `:336` to D-5's predicate (holds every published cut **or** carries a recorded refusal for each it lacks), reading the census -- a `local` family needs no census, because the catalogue is the authority on what it publishes; delete `mostRecentlyFetched` and the `storedByFamily` fold; reshape `localByFamily`; rewrite the comment at `:264-272` to state the new model -- the fold is wrong in kind, not in its tie-break.
- [x] `folio-designer/src/App.tsx` -- write every fetched cut through `keepOnThisMachine` and write the family's census alongside them; make `installFamily` accept a `stored` row whose census is short and fetch **only** the missing cuts (D-3), and refuse-but-keep on a partial write (D-1); update `scriptsOfSource`, the specimen/preview byte reads, `addFamilyToDocument` and `embedInstalledFamily` to take the Regular out of the set -- the document path must behave exactly as it does today.
- [x] `folio-designer/src/font-browser-model.ts`, `folio-designer/src/FontBrowser.tsx` -- follow the reshaped type; keep the row and its install count family-counted.
- [x] `folio-designer/src/font-source.test.ts` -- correct the abort-chain call-count table and the success-path request count to the new chain length, and cover the per-cut refusals in the matrix (variable cut, unreadable media type, stalled cut) -- the counts are a bound to re-measure, not to delete.
- [x] `folio-designer/src/font-index.test.ts` -- replace the two `mostRecentlyFetched` tests with assertions that a family holding four faces is offered once and each cut resolves to the record whose `style` matches, stable across arrival order and same-day `fetchedAt`.
- [x] `folio-designer/src/App.font-store.test.tsx`, `folio-designer/src/font-browser-model.test.ts`, `folio-designer/src/FontBrowser.test.tsx`, `folio-designer/src/font-provenance.test.ts` -- extend the upstream stub to serve the cut files, correct the exact store-listing assertions to the new record counts, and update the `FamilySource` fixtures.

- [x] `folio-designer/src/font-source.ts` -- classify every per-cut failure as permanent or transient at the point the failure is known, defaulting to transient when the shape is not confidently permanent (D-2 amended) -- derived from the failure, never guessed by the caller.
- [x] `folio-designer/src/font-store.ts` -- carry the classification on the refusal record and validate it; `censusIsComplete` settles a cut only on a **permanent** refusal (D-5 amended).
- [x] `folio-designer/src/font-index.ts` -- `familyIsInstalled`'s stored arm must not make a held family unusable: a family whose faces are held stays in AVAILABLE LOCALLY whatever its census says (D-8).
- [x] `folio-designer/src/App.tsx` -- pass `held` at both remaining `fetchWebFamily` call sites (`:1120` specimen, `:2967` re-embed refetch) so browsing costs one body read per family again; refresh the listing on a failed census write as the per-cut write-failure path already does.
- [x] `folio-designer/src/font-store.ts` -- add `database.onversionchange` so an older tab closes its connection rather than blocking the v1→v2 upgrade for a whole session.
- [x] tests -- cover each new matrix row: a transient refusal retried on a later pick, a permanent refusal settling the cut, and a census-less held family still usable offline.

**Acceptance Criteria:**
- Given a family publishing all four cuts, when the author installs it, then the store holds four records for that family, each carrying its own `style`, `source`, `licence`, `licenceText` and `copyright`, and the document is unchanged.
- Given a family publishing only a Regular, when the author installs it, then exactly one record is written and no refusal is raised.
- Given a family whose non-base cut is a variable font or an unreadable media type, when the author installs it, then the family installs without that cut and the other cuts are unaffected.
- Given a family publishing no upright weight-400 static face, when the author picks it, then the install is refused before any face bytes are kept.
- Given a store holding four faces of one family, when the font browser lists families, then that family appears exactly once, and the face chosen for each cut is the one whose `style` matches — not the lexicographically smallest key, and not arrival-order dependent.
- Given a family installed with all four cuts, when the author applies it and presses **B**, then the behaviour is identical to today's: the Regular paints and the existing absence sentence stands. This story does not change it.
- Given a v1 store holding face records, when this build opens it, then it upgrades to v2 and every one of those face records is still readable — no wipe, no refetch.
- Given a family whose italic fetch failed, when the browser lists families, then it reads installed (the refusal is recorded for the cut it lacks) and is not offered for install again — so a family whose cut keeps failing does not re-offer forever.
- Given a family installed before this change, holding only its Regular with no census, when the browser lists families, then it reads incomplete and is offered for install again, and picking it fetches only the cuts it lacks.
- Given a family publishing only a Regular, when it holds that Regular, then it reads installed and complete — the 947-of-1,274 common case never re-offers.
- Given a family whose Bold fetch stalls, when the author picks that family again later, then the Bold is fetched — the stall sentence's "try the pick again" is true for a non-base cut.
- Given a family whose Bold carries an `fvar`, when the browser lists families, then it reads installed and is never offered again — a permanent refusal still closes D-5's loop.
- Given a family installed before this story, holding its faces with no census, when the machine is offline, then the family is still listed under AVAILABLE LOCALLY and can still be applied.

## Implementation Notes

### Review loop 1 (2026-09-20)

**A refusal now carries whether it is settled, and the classification is derived where the failure is known.** `RefusedCut`/`FamilyCutRefusal` gained `permanence: 'permanent' | 'transient'`, decided inside `fetchCut` — permanent for a variable `fvar`, an unreadable media type and a 404/410; transient for a stall, a dead connection, a 5xx, a 403 and **a body that will not parse**. That last one looks wrong and is not: the commonest real producer of "not a static TrueType sfnt" is a captive portal's 200 HTML login page, which is the most transient failure in the set, and D-2's default rule settles it. `censusIsComplete` settles a cut only on a permanent refusal; `soundCensus` rejects a census carrying a permanence it does not recognise, rather than admitting it and guessing.

**`familyIsInstalled` and `familyIsComplete` are two predicates now (D-8).** They had been fused, and the fusion made a census-less family — anything installed before this story — vanish from `AVAILABLE LOCALLY`, which offline makes a font already on the machine unusable. `familyIsInstalled` answers *can these bytes be used* (stored → `faces.length > 0`) and is what the family control filters on; `familyIsComplete` answers *is there anything left to fetch* and is what `FontBrowser.tsx`'s row state and `installFamily`'s re-pick guard ask. One row can now honestly be usable **and** offered for the cuts it lacks.

**`fetchWebFamily`'s fourth argument is `skip`, not `held`.** Two callers want one face rather than a family's set — the browser's specimen (set in the Regular) and the re-embed refetch (whose document carries the Regular alone) — and both now pass `cutsBesideTheRegular`. Browsing a page of twelve rows is back to twelve body reads instead of as many as forty-eight. Naming the parameter for what it *does* rather than for one caller's reason is what makes that second use honest.

**`recordFamilyCensus` refreshes the listing on both exits**, mirroring the per-cut write-failure path three lines away: the faces landed before that call, so returning the reason without refreshing left them written to the machine and invisible in the designer.

**`openFontStore` answers `onversionchange` by closing.** Unanswered, a tab on an older build holds its v1 connection, the newer tab's `open` sits in `onblocked`, and that tab runs with no store at all until the old one is closed by hand — a real condition the moment a release bumps `databaseVersion`, which this story does.

**Both amendments are red-proved.** Reverting `censusIsComplete` to settle on any refusal fails exactly three cases including "retries a transiently refused cut on a later pick"; reverting `familyIsInstalled`'s stored arm to the completeness check fails exactly three including "still lists and applies a family installed before this story, with no network at all".

### Original pass

**The census record carries no `held` list, and the omission is D-7's own reasoning applied to itself.** D-7 names three things for the census to hold — "what upstream publishes, what is held, and what was refused and why" — and the record as built holds two of them: `published` and `refused` (plus `recordedAt`). Which cuts this machine holds is read off the **face records**, every one of which already carries a `style`. The decision's stated ground for choosing a separate store over a per-face field was that it is *one authority on one fact*, and a `held` list here would be a second authority on a fact the face set already carries — the N-copies-that-can-disagree shape the decision rejected. It would also fail in a direction the reader can see: the face store **self-heals by dropping** a record it cannot verify, and a `held` list would go on claiming a cut whose bytes had just been dropped. The obligation that came with the approval is discharged in full — the three states D-5's predicate and story 2's panel need are all distinguishable (`published` ∧ `refused` = the fetch failed; ¬`published` = upstream publishes no such cut; `published` ∧ ¬`refused` ∧ no face record of that style = never attempted) — and nothing in the census is write-only. **This was flagged for the owner rather than absorbed silently, and was RATIFIED into D-7 on 2026-09-20 (ruling 2): the census carries no `held` field, and that must not be "restored".**

**`FamilySource` holds `faces` on both installed arms, and the cut is resolved by `style`.** `regularCutOf` (exported from `font-index.ts`) is the single resolver: `faces[0]` would have made "which face is the Regular" a fact about the store's family-then-**key** sort, which is the content hash, which is arbitrary — the same silent substitution the deleted `mostRecentlyFetched` fold committed by way of its same-day tie-break. `sourceScripts` was hoisted into `font-index.ts` for the same reason: `App.tsx`'s `scriptsOfSource` and `font-browser-model.ts`'s `browserRows` were two copies of one ternary and are now one function, so the panel and the dialog cannot describe a family's coverage two different ways.

**The two store listings are read as a pair, not in sequence.** `familyIsInstalled` answers from both the face set and the census, so a face listing landing one render ahead of its census makes every stored family read *incomplete* for that render — the family control drops it out of `AVAILABLE LOCALLY` and puts it back, a flicker on every load. `Promise.all` over `list()` and `listCensus()` lands them in one render. This was measured, not predicted: seven `App.font-store.test.tsx` cases failed on exactly that race before the pair read.

**`keepOnThisMachine` gained a `refresh` flag, for cost and not correctness.** Every refresh changes `machineFaceListing`, which tears down and rebuilds the preview registration — reading every stored face's *bytes*. Refreshing per cut would do that four times for one install, so `installFamily` writes the set with `refresh: false` and refreshes once, after the census. The refusal path (D-1) refreshes too, because the cuts that landed are on this machine and the designer must go on saying so.

**A re-pick that needs nothing stops after the metadata.** `fetchWebFamily` returns early — no licence read, no body read — when every published cut is already held. This is the migration path and it is the **common case**: 947 of the 1,274 offered families publish a Regular and nothing else, so every one of them installed before this story holds everything it publishes and is missing only a census. The licence text is what travels with a *new record*; reading it when no record will be written buys nothing, and the terms on the held records are untouched.

**The cuts are fetched sequentially, not with `Promise.all`.** Four concurrent cross-origin body reads of up to 24 MB each would replace a bounded, cancellable, stated sequence with a burst nothing in the module can reason about — and would make the abort budget, the refusal order and the request count untestable. The cost is latency on a path the author already waits on.

**`App.font-store.test.tsx`'s existing Kanit fixture was left publishing one cut.** The story's task list asks to "extend the upstream stub to serve the cut files" and "correct the exact store-listing assertions to the new record counts". The existing fixture's `METADATA.pb` declares only an upright 400, so it already exercises the **947-of-1,274 common case** and no listing count moved. Rather than churn ~20 assertions to re-pin numbers that would say the same thing, a new `describe` block drives a **four-cut** Kanit through the real font browser and asserts the four records, their distinct keys, styles and `source` paths, the census, the silent per-cut skip, D-3's fetch-only-what-is-missing, and D-1's refuse-but-keep. `font-source.test.ts`'s fixture **was** extended — its metadata already published an italic-400 it declined to serve, which would have made every request count a measurement of a 404.

**`sfntWithNames` gained a `withFvar` option.** `faceIsVariable` reads the presence of the `fvar` **tag** in the table directory and nothing behind it, so a well-formed empty `fvar` is the honest fixture for "this face declares itself variable"; building real axis records would be building a richer claim than the code makes.

**`font-store.test.ts`'s `writeRaw` now opens unversioned.** It named version `1` explicitly, which raised `VersionError` the moment `openFontStore` created the database at version 2 — three unrelated cases failed for a reason that had nothing to do with what they assert.

**Matrix audit (step-03) found one uncovered row, and it was closed rather than waived.** The matrix's *Partial family* row — "family publishes Regular + Bold only → two records; no italic record, no placeholder" — was covered at the **selection** level (`publishedCuts` over the Kanit fixture) but at no point at the **store** level: every install test drove a four-cut upstream. A `twoCutUpstream` fixture and `writes only the cuts upstream publishes, and no placeholder for the ones it does not` now assert the two records and, more importantly, that the census does **not** list the two cuts upstream never published — a phantom `published` entry would read as *never attempted* for ever and re-offer the family on every render, which is the exact loop D-5 exists to close. The test was **red-proved**: mutating `publishedCuts` to stop dropping an unpublished cut fails it, and only it.

## Spec Change Log

- **2026-09-20 — D-7 implemented without a `held` field on the census record.** Reasoned in Implementation Notes above. The three states the approval's own obligation names are all still distinguishable; what changed is that *held* is read from the face records rather than duplicated into the census. Flagged for the owner because D-7 sits inside the frozen block and names `held` in words.
- **2026-09-20 — `font-source.ts`'s `regularFilename` was replaced rather than kept as a wrapper.** The Code Map calls it "the narrowing to generalise"; `publishedCuts` is that generalisation and a surviving one-cut wrapper would have been a second way to ask one question. Its test was replaced by a named successor, not dropped.
- **2026-09-20 — `scriptsOfSource` / `browserRows`' coverage ternary were merged into `font-index.ts`'s `sourceScripts`.** Not in the task list; forced by the reshape, which would otherwise have required the same three-arm narrowing to be rewritten in two modules that must agree.

## Review Triage Log

**Pass 1 — 2026-09-20.** Three layers ran (blind-hunter, edge-case-hunter, verification-gap). Every finding below was verified at its cited location before a verdict was rendered.

### Routed: intent_gap (triggers loopback — root cause is inside `<frozen-after-approval>`)

| Verdict | Finding | Evidence |
|---|---|---|
| **high** | A transient per-cut failure is recorded as a permanent refusal and nothing ever retries it. | Verified at `font-source.ts:694`: `else refused.push({ style: entry.cut, reason: fetched.reason })` catches **every** non-success — a stalled body and offline land in the same list as a variable `fvar` and a 404. `censusIsComplete` (`font-store.ts:191`) counts any recorded refusal as settling the cut, so `familyIsInstalled` reads the family complete, the browser stops offering it, and no path in the designer refetches that cut. `recordedAt` is written and validated but **no production code reads it**. Sharpest proof: `stalledRefusal` (`font-source.ts:456`) tells the author *"Try the pick again if you like"* — which this change makes false for a non-base cut. |
| **high** | Same root cause, second face: a stored family with no census reads not-installed, so offline it becomes **unusable**, not merely re-offerable. | Verified: `familyIsInstalled`'s stored arm (`font-index.ts:431`) returns `false` without a census; `App.tsx:5540`'s `onThisMachine` filter uses it, so the family leaves AVAILABLE LOCALLY and `embedInstalledFamily` can no longer be reached for it. The owner accepted that incomplete families "move back into the installable group"; offline they move out of reach entirely, which is a strictly worse outcome than the one accepted. |
| **medium** | The "self-healing" claim at `App.tsx:2784` is false for any cut carrying a refusal. | Same root cause. A refused cut whose bytes are later dropped by the store's self-heal still reads complete, so it is never refetched. |

**Why intent_gap and not patch or bad_spec.** The behaviour is exactly what the frozen block specifies: the matrix row *"Bold stalls → Regular installs; Bold skipped and recorded refused"*, D-2's **two-way** distinction (upstream-refused vs never-attempted, with no third state for a transient failure), and D-5's *"or carries a recorded refusal for each cut it lacks"*. The code is faithful; the captured intent is incomplete. There is **more than one defensible reading** of what the owner wants — (i) transient failures are not recorded, so the family stays incomplete and retries (reopening the re-offer loop for offline authors), (ii) refusals carry a class and `recordedAt` drives a retry policy, (iii) accept it and give the author an explicit "look for missing cuts" action — so intent may not be inferred.

### Routed: patch (moot this pass — code will be re-derived after the loopback, but carried so they are not re-found)

| Verdict | Finding | Evidence |
|---|---|---|
| medium | `browserSpecimenBytes` (`App.tsx:1120`) and `embedInstalledFamily`'s refetch (`App.tsx:2967`) call `fetchWebFamily(source.family)` with no `held`, then discard all but the Regular. | Verified: `:2791` passes `held`, these two do not. Browsing a page of web rows goes from one body read per family to up to four. Found independently by all three layers. Fix is one argument. |
| medium | A failed census write leaves the just-written faces invisible. | Verified in `recordFamilyCensus` (`App.tsx`): `refreshStoredFaces()` is called only on success, while the per-cut write-failure path three lines away does refresh. |
| medium | No `database.onversionchange` handler, so a v1 tab blocks the v2 upgrade for a whole session. | Verified: `grep -c onversionchange font-store.ts` = 0. Dead risk before this story (there was only ever version 1); this change makes the upgrade real and the `onblocked` reject reachable. |
| low | `sourceScripts` falls back to `cuts[0]?.scripts`, the positional resolution `regularCutOf`'s own doc refuses at length. | Verified in `font-index.ts`. |
| — | Four verification gaps filed pre-verified by the verification-gap layer: the re-embed `held` adoption, the partial-install `refreshStoredFaces`, a failed census write, and the 947-of-1,274 migration case — all tested only at the resolver, not at the `App.tsx` seam. | Each names a mutation that leaves the suite green. |

### Routed: defer (not this story's problem)

| Verdict | Finding | Evidence |
|---|---|---|
| low | `readHeldLocalFamilies` (`held-local-faces.ts:93-95`) marks a family held when **any** one catalogue face is cached — the same any-wins shape `localByFamily` was reshaped to remove. | Pre-existing and not live: the catalogue carries one face per family today. It becomes a real defect in story 3, which supplies the cuts. |
| low | `addableFamilyCount` counts catalogue **faces**, not families. | Same condition — only wrong once story 3 lands multi-cut catalogue rows. |
| low | No census removal path; `remove(key)` does not consider one, so a census outlives every face it describes. | Real lifecycle gap in an API D-7 declares inherited by stories 2-4. Worth writing down rather than leaving to omission. |
| medium (unverified) | Two cuts with byte-identical files collapse to one content-addressed record; `published` then names a cut that is neither held nor refused, so the family re-offers for ever. | Would need an upstream family shipping two identical cut files to confirm; plausible but not demonstrated on any real family. |
| low | `FamilyCensus.published` and `fetchWebFamily`'s `held` are `ReadonlyArray<string>`, not the `FaceCut` closed set; `soundCensus` accepts any non-empty string and never checks a `refused[].style` is in `published`. | Real looseness at the IndexedDB boundary; routes with the API-shape work stories 2-4 inherit. |

### Rejected

| Verdict | Finding | Refutation |
|---|---|---|
| false | `soundCensus` admits `published: []`, making `censusIsComplete` vacuously true. | Unreachable from the product: `publishedCuts` can only return empty when metadata declares no upright 400, and the base refusal at `font-source.ts:542` fires first, so no install ever writes such a census. Guarding a state never shown reachable adds a branch for nothing. |
| low | `transact` names the census store on every face transaction, widening every lock. | Correctness-neutral; no named harm beyond lock scope in a single-tab, low-contention store. Fix adds a parameter to every call site. |
| low | One concept carries three field names (`PublishedCut.cut`, `FamilyCutRefusal.style`, `FetchedFace.style`). | Naming consistency with no named failure; rejected here and noted for the stories-2-4 API pass. |


## Verification

**Commands:**
- `cd folio-designer && npm run typecheck` -- expected: clean. **Result: clean.**
- `cd folio-designer && npm run lint` -- expected: **8** pre-existing `only-export-components` warnings, no new ones. (The "4" this spec first carried was stale; 8 was measured at `cfbe5da` by stashing this story's changes and re-running.) **Result: 8 warnings, all `only-export-components`, and 8 is the BASELINE count — measured by stashing this change and re-running at `cfbe5da`. The spec's "4" was stale; no new warning was introduced.**
- `cd folio-designer && npm test` -- expected: full Vitest suite green. **Result: 89 files, 2,049 tests, all passing. Baseline at `cfbe5da`: 89 files, 2,032 tests. Test-name set diffed against the baseline run (JSON reporter, both directions): 20 added, 3 removed, and each of the 3 is a named replacement rather than a deletion —**
  - `font-index.test.ts` "offers the most recently fetched of two stored faces of one family…" and "breaks a same-day tie on the key rather than on arrival order" → replaced by "offers a family holding four cuts exactly once, with each cut resolving by its own style" and "keeps every cut of a family whose faces all carry the same fetch date". The deleted pair pinned `mostRecentlyFetched`, which this story deletes as wrong in kind.
  - `font-source.test.ts` "reads the Regular filename from the style:\"normal\" weight:400 entry…" → replaced by "reads each cut's filename from its own style/weight entry…", which asserts the same rule over all four cuts.
- `cd folio-go && go test ./...` -- expected: unchanged from baseline (this story touches no Go). **Result: unchanged. One pre-existing failure, `internal/text` `TestCorpusMeetsP6ExerciseFloors/P6g (opaque names)` — "floor not met: got 7, need >=20" — reproduced BYTE-IDENTICALLY at the baseline commit with this change stashed. Every other package passes. Not this story's.**

**Red proofs:**
- The v1 → v2 upgrade case was red-proved: setting `databaseVersion` back to `1` fails "upgrades a v1 store to v2 with every face record it already held still readable" and nothing else. The case builds a real version-1 database by hand — two object stores, a real face record and its bytes — before handing it to `openFontStore`, so it cannot be satisfied by a fresh database.
- The four-cut install case cannot be satisfied by one face written four times: the four fixture cuts carry four different `nameID 0` strings, so they hash to four different keys, and the case asserts `new Set(keys).size === 4`.

**Manual checks:**
- `folio-designer/e2e/font-browser.spec.ts:133-154` — **measured rather than predicted, as the spec instructed. It still passes and has been left exactly as it is.** `npx playwright test e2e/font-browser.spec.ts`: 6 passed, including "staging several families states what is about to be installed, and Escape discards it" with its `Install 2 on this machine` assertion. D-4 does not move it, for the reason the spec anticipated: `confirmLabel(staged)` counts families the author has **ticked**, and both staged rows are `web`-tier rows that are addable under the old predicate and the new one alike. A fresh browser holds no stored faces at all, so the stored arm — the only arm D-4 changes — is empty in that run.
- `npm run scan:font-hosts` and `npm run scan:host-fonts`: 0 occurrences, floors met. `npm run test:e2e:compile`: clean.
