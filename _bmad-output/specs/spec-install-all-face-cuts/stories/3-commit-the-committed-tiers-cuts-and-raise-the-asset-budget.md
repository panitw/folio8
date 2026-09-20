---
title: "Commit the committed tier's cuts and raise the asset budget"
type: 'feature'
created: '2026-09-19'
status: 'done'
route: 'dispatch'
review_loop_iteration: 1
baseline_commit: '8038ad8d77db8c531492b10f07cb32a1a08292a6'
context: ['{project-root}/_bmad-output/specs/spec-install-all-face-cuts/SPEC.md']
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The committed catalogue is one upright Regular per family, by rule and by assertion
(`font-catalogue.test.ts:461-481`, `build-wasm.mjs:107-112`). 30 of its 31 families cannot be
reached from the web tier at all — 28 carry variable axes and `addableFromTheWeb`
(`font-index.ts:149`) filters those out; Inter Display and Source Serif 4 Display are absent from
the index entirely (all three counts measured, not quoted). So after stories 1 and 2 give the
designer a face-set model and a per-cut embed, an offline author would still lose bold on 30
families an online author would get, and CAP-3 would be false.

**Approach:** The committed tier grows from 31 faces to the cuts those 31 families actually
publish — **112 declared cuts across the 31 families, of which 78 are new files to commit**
(25 families × 4, 6 families × 2; Roboto's three cuts are already shipped and are not redeclared).
Each new cut is an upstream static **copied byte for byte out of the same project archive its
family's Regular came from** — the archive URL, its SHA-256 and the path inside it are already
pinned in that family's `NOTICE.md`. Each becomes a `font-catalogue.json` row carrying its own
`style`, its own directory, its own LICENSE and NOTICE.md, and its own deferred asset — the same
shape a Regular row already has. The offline release's `maximumCacheAssets` rises from 90 to fit
the measured result, derived and not retyped, with `warnCacheAssets` moved with it. The blocking
core tier does not move.

**Where this sits.** Planned against `8038ad8`, with stories 1, 6 and 2 landed: story 1's
family-face-set model (`localByFamily` is already a `ReadonlyMap<string, ReadonlyArray<CatalogueFace>>`),
story 6's multi-command transaction, and story 2's per-cut chain entry are what make a multi-cut
catalogue consumable. The inventory below was re-verified against the upstream archives themselves
after those landed, not against the `font-index.json` snapshot.

## Boundaries & Constraints

**Decisions (settled — do not revisit):**
- **The cuts are COPIED, never derived.** Each new face is an upstream static taken byte for byte
  from the same project archive the family's Regular came from, verified against that archive's
  pinned SHA-256. `tools/fontgen/instance_faces.py` is **not** the catalogue's pipeline — it drives
  seven **engine** faces, all Noto, into `folio-go/fonts/` — and **must not be extended into one**.
  No instancing, no variable sources, no fontTools. Every new `NOTICE.md` records *"copied
  unmodified, no derivation"*, like the 31 Regulars before it. (SPEC.md constraint, corrected.)
- **Roboto's cuts are NOT declared in the catalogue.** `Roboto Bold`, `Roboto Italic` and
  `Roboto Bold Italic` already ship as **core** assets and keep serving that family. Do not emit a
  second dist asset with byte-identical content, and do not retire the core copies — that would
  move the 30/30 core pin SPEC.md forbids. Roboto is therefore the one family whose cuts come from
  a different place than every other family's: **make that join explicit and commented**, not
  incidental, wherever the browser or the store reads a family's cuts.
- **`maximumCacheAssets` 90 → 168, `warnCacheAssets` 82 → 160.** 158 measured assets plus the same
  10-slot reserve today's 90 carries over 80, warning eight below the ceiling. **Both must be
  re-derived from a real emitted manifest before they are pinned**, each on its own line in the
  `^const <name> = <digits>$` shape the reader requires, with the rationale beside it.
- **One story, the whole batch.** CAP-3 lands complete; all 78 faces in this change.
- **The embed path resolves a local cut from the CATALOGUE, not from the face store** (owner ruling,
  2026-09-20, option (b) of two). The release cache stays the authority for committed faces and the
  IndexedDB store stays the authority for fetched web faces; the embed path learns to ask the right
  one. A second copy of a committed face in the store is refused — `App.tsx:2754-2756`'s *"two
  answers to one question"* survives this change rather than being its casualty. Checked before
  building: `CatalogueFace` already carries all six fields `embeddedFontRecord` requires (family,
  style, licence, licenceText, copyright, source) plus `url`, and `commitPropertiesEmbeddingCuts` is
  already `async` and already awaits per-plan reads — so (b) fits the existing shape.
- **A committed cut's bytes come from the release cache, never the network.** `fetch(cut.url)` for a
  held catalogue face is served by the service worker, so an author's keystroke does not depend on a
  connection. A cut that is NOT cached must be refused by name with a remedy, in the idiom of the
  existing store-miss sentence — never silently fetched from upstream.
- **The shipped catalogue IS the local tier's census.** `cutAbsenceState` must answer all four states
  for a local family: presence in the catalogue is certain and absence from it is genuine, so it must
  stop returning `unpublished` for a cut the release ships.

**Always:**
- **A row declares the BASE family plus its `style`; the CSS family name is DERIVED.** A row is
  `{ family: "Inter", style: "Bold" }`, and the emitter derives the rule's family name
  (`Inter Bold`, bare `Inter` for Regular) exactly as `shipped-face-cuts.ts:77-83` does. This is
  forced from both ends: `font-catalogue.test.ts:467` asserts the binary's own family equals
  `face.family`, and a bold cut's name[1] **is** the base family (`build-wasm.mjs:567-571`) — so
  `family: "Inter Bold"` would red it; while `canvasFaceAssets` (`:614-624`) and the `@font-face`
  rules need the cut name. Consequence: the uniqueness key at `build-wasm.mjs:313` and
  `font-catalogue.test.ts:337` must become **(family, style)**, not family, and the `:294` seed from
  `shippedFamilies` must be re-derived against the same pair.
- **No `font-weight` / `font-style` descriptors — still.** Per-cut CSS family names are what make
  that survivable; `build-wasm.mjs:555-563` and `font-catalogue.test.ts:524-541` stay as they are.
  Four rules under one bare family name with no descriptors would let the last rule win silently,
  and `:615` counts array entries rather than map keys, so it could not see it.
- **The exact filename is the contract; a prefix is not.** Several archives hold a *different
  family* in the same directory whose filename shares the stem: `OpenSans-Condensed*`,
  `NotoSerif-Condensed*`/`-ExtraCondensed*`/`-SemiCondensed*`, `UbuntuSansCondensed-*`,
  `RobotoCondensed-*` beside `Roboto-*`, `CascadiaCodeNF-*`/`-PL-*`, `JetBrainsMonoNL-*`,
  `Literata7pt`/`36pt`/`72pt-*`, `NotoSansThaiLooped-Condensed*`. Match the exact name or the build
  silently commits the wrong face. **The three Adobe families spell the slope `-It` / `-BoldIt`**,
  not `-Italic` / `-BoldItalic`.
- **Every family-keyed collapse must be re-keyed or proven safe.** Story 1 already fixed
  `font-index.ts:183` (`localByFamily` is now a face-set map) — **do not re-fix it**. Still live:
  `font-index.ts:247` `addableFamilyCount = webFamilies.length + catalogueFaces.length` counts
  faces, and `held-local-faces.ts:93-94` marks a family held when **any one** of its faces is
  cached, so a family with only its Regular cached reads as complete.
- **`absent-face-recovery.ts` is not modified — it is re-proven at the new scale.** Story 1 fenced
  it, and its `absentFaceNames` substring match is **already** defended against the base/cut
  ambiguity: `:73-88` sorts candidates longest-first and blanks each hit, with `Noto Sans` vs
  `Noto Sans SC` as its own worked example. `canvasFaceAssets` already carries `Noto Sans` beside
  `Noto Sans Bold` and `Roboto` beside `Roboto Bold`, so the hazard is pre-existing, not introduced
  here. What this story changes is the candidate set, 44 names to ~122. Add a test at that scale;
  do not redesign the function.
- **The core BYTE ceiling is live and thin.** `maximumCoreCacheBytes = 6553600`
  (`release-payload.ts:179`) has **~155,543 Brotli bytes of headroom** at the last build.
  `src/generated/font-catalogue.ts` inlines one ~4 KB licence text **per face**
  (`build-wasm.mjs:374`) plus a per-face `copyright` and `source` string, and that module is
  bundled into the **core** tier. 78 more faces is ~312 KB of raw bundle text. The licence texts
  are near-duplicates over three SPDX ids so Brotli should crush them, but the `source` strings are
  unique. **Measure it; do not assume it.** If it breaches, the remedy is deduplicating the licence
  texts by SPDX id — `build-wasm.mjs:341-342` already notes 31 faces emit 31 texts over three
  identifiers — **never** moving the ceiling, which guards the first-load screen.
- **A family installs the cuts it has.** Six families publish no italic (Fira Code, Noto Sans Thai
  Looped, Noto Serif Thai, Oswald, Roboto Slab, Space Grotesk). Absence is a first-class answer, as
  `shipped-face-cuts.ts:59-64` already states, and CAP-2's sentence then tells the truth.
- **The budget numbers are DERIVED from a real `npm run build`, never retyped.**
  `offline-release-contract.mjs:204-208` matches `^const <name> = <digits>$` with exactly one live
  match and throws on 0 or 2. Each constant keeps its own line in that exact shape, with the
  rationale written beside it the way the 65→90 raise did.
- **The core tier pin does not move.** 30/30 at `release-payload.ts:150-151`; catalogue faces are
  `deferred` at `offline-release-contract.mjs:126`. New cuts must cost nothing at startup.
- **`CORE_CATALOGUE_FACE_IDS` stays `['roboto']`.** The `\.` after the id alternation
  (`offline-release-contract.mjs:116`, rationale at `:119-122`) is what keeps `robotocondensed`,
  `robotomono` and `robotoslab` out of the core arm; any new Roboto-prefixed id must stay out too.
- **The retired assertion is REPLACED, not deleted.** `font-catalogue.test.ts:461-481` becomes a
  per-row claim: the binary's subfamily, `usWeightClass`, `fsSelection` bold/italic bits, `macStyle`
  and `italicAngle` must match the row's declared `style`. `:476` (no variable tables) and `:478-479`
  (glyf/`.ttf`) survive unchanged. The working pattern already exists at `:654-760`, including
  `it('tells a Regular from the Bold cut of the same family')` at `:736`.
- **Every new face carries its own licence.** One `LICENSE*` and one `NOTICE.md` per face
  directory, with the six recorded rows `build-wasm.mjs:420-436` and `font-catalogue.test.ts:377-403`
  parse, including the shipped-file sha256 and byte size.
- **`npm run build` does not run Vitest.** It is `build:wasm && tsc -b && vite build && build:offline
  && verify:offline`. A drifted constant plus a tie test in Vitest leaves the build green.

**Never:**
- Do not create a second copy of Roboto Bold, Roboto Italic or Roboto Bold Italic. They already ship
  as committed directories (`public/fonts/roboto-bold/`, `-italic/`, `-bolditalic/`) **and as core
  release assets** (`/assets/roboto-bold.*`, `roboto-italic.*`, `roboto-bold-italic.*`), and the
  ruling is that those keep serving the family.
- Do not add any face to the core tier, to `CORE_CATALOGUE_FACE_IDS`, or to `public/templates/starter.folio`.
- Do not add a `font-weight` or `font-style` descriptor to any emitted `@font-face` rule.
- No new weight beyond the four cuts, no variable fonts, no synthetic bold or oblique, no CJK
  catalogue change (Noto Sans SC stays on the shipped-face path).
- Do not change the `.folio` format, the chain mechanism, or the `fonts` map syntax.
- Do not touch the Add font dialog's pre-pick **cut display** — that is story 5's work. Correcting a
  sentence that has become false is not that, and is in scope.
- ⚠ **FENCE MOVED BY THE OWNER, 2026-09-20.** This line previously read *"Do not touch the document,
  any engine command, or the Add font dialog's pre-pick display"*. Review found that the fence
  excluded the very path CAP-3 needs: a committed family's cut could never be embedded, and the
  panel said *"No bold face in this family"* about a bold this release ships. The owner authorised
  opening the embed path in this story, ruling that **the story does not close until pressing B on a
  committed family paints the bold with no diagnostic.** Recorded here rather than widened quietly.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Four-cut family | Inter: upstream publishes 400/700/400i/700i | Four catalogue rows (`inter`, `interbold`, `interitalic`, `interbolditalic`), all `family: "Inter"` with distinct `style`; four deferred assets; four `@font-face` rules under the derived names `Inter` / `Inter Bold` / `Inter Italic` / `Inter Bold Italic` | N/A |
| Family offered once | Four Inter rows in `catalogueFaces` | The Add font dialog lists `Inter` exactly once (`font-index.test.ts:189-197`), `addableFamilyCount` counts it once, and `localByFamily` resolves each cut by `style` rather than last-wins | N/A |
| Core bundle grows | 78 more inlined licence/copyright/source strings in `font-catalogue.ts` | Core Brotli stays at or under `maximumCoreCacheBytes` 6,553,600 | `generate-offline-release.mjs:222-224` throws naming the weight and the ceiling |
| Italic-less family | Oswald: upstream publishes 400/700, no 400i | Two rows only; `Oswald Italic` is never declared, never fetched, never offered | N/A |
| Row style disagrees with bytes | A row declares `Bold`; the binary reports subfamily `Regular`, `usWeightClass` 400 | `font-catalogue.test.ts` reds naming the row, the declared style and the subfamily read from the bytes | Test failure |
| Two rows, one (family, style) | Two rows both declare `Inter` / `Bold` | The re-keyed `build-wasm.mjs:313` throws before any asset is emitted | Build failure, naming family and style |
| Cuts added, budget not moved | Release emits >90 assets | `verify-offline-release.mjs:118` fails against the derived `maximumCacheAssets` | Build failure, count and bound named |
| Budget line reformatted | `const maximumCacheAssets = 168` split or commented | `readDeclaredConstant` finds ≠1 live match and throws | Build failure, naming the reader |
| Core tier unchanged | 78 new deferred assets emitted | `coreAssetCount` stays 30; `maximumCoreCacheBytes` (6,553,600) untouched | N/A |
| Catalogue row count drifts from emitted assets | `font-catalogue.json` and the release disagree | `generate-offline-release.mjs:207` throws naming both counts | Build failure |

</frozen-after-approval>

## Code Map

**The catalogue's declaration and emission**
- `folio-designer/font-catalogue.json` — 31 rows, keys `id, directory, file, family, licence, scripts`. **No `style` field**; this story adds it.
- `folio-designer/public/fonts/<dir>/` — 44 directories today, 31 of them catalogue; each is exactly 3 files (one `.ttf`, one `LICENSE*`, one `NOTICE.md`). The other 13 are the shipped/canvas faces, including `roboto-bold`, `roboto-italic`, `roboto-bolditalic`.
- `folio-designer/scripts/build-wasm.mjs` — the only producer. `:107-126` the "catalogue stays Regular-only" rationale (**rewrite**); `:296` required-field list (**add `style` + a closed-set check**); `:307` id shape `^[a-z0-9]+$`; `:313` family-uniqueness, seeded at `:294` from `shippedFamilies` (**keep; per-cut family names satisfy it**); `:317` `fingerprint()` → one row = one asset = one cache slot; `:420-436` the four NOTICE rows; `:460` **`style: "Regular"` hardcoded — the stamp to replace**; `:555-563`/`:583-584` the no-descriptor `@font-face` emitter; `:614-624` `canvasFaceAssets`, a `Map` keyed by family (silently collapses if two rows share a family name — `:615` counts array entries, not map keys, and would not catch it); stale "31 faces" prose at `:335`, `:341`, `:374`, `:595`; stale slot arithmetic at `:118`, `:449-450`.
- `folio-designer/src/generated/font-catalogue.ts` — generated; `CatalogueFace` at `build-wasm.mjs:455` already types `style: string`, so no type change is needed.

**The budget**
- `folio-designer/src/release-payload.ts` — `:65` `minimumCacheAssets = 10`; `:72` `maximumCacheAssets = 90` with its 65→90 rationale at `:66-71`; `:91` `warnCacheAssets = 82` with `:73-90`; `:150-151` core pins 30/30 (**do not move**); `:179` `maximumCoreCacheBytes = 6553600` (unaffected — new faces are deferred).
- `folio-designer/scripts/offline-release-contract.mjs` — `:19` `isCatalogueAssetUrl`; `:102` `CORE_CATALOGUE_FACE_IDS = ['roboto']`; `:111-115` module-load guards holding each id to the JSON; `:116` the regex interpolation and its `\.` guard; `:126` the deferred classification; `:204-208` `readDeclaredConstant`, the line-anchored reader; `:224-247`/`:260-293` the coherence checks (warn ≤ max, warn ≥ min, min ≤ max).
- `folio-designer/scripts/generate-offline-release.mjs` — `:205-207` the catalogue-length tie; `:237` `brotli.catalogue.familyCount` (**becomes a face count — rename or it lies**); `:222-224` the core byte ceiling.
- `folio-designer/scripts/verify-offline-release.mjs` — `:118-119` the release bound; `:138`/`:70-75` the approach warning; `:171-174` the core pin; `:409` the catalogue familyCount tie; `:452-479` canvas-face-map drift.
- Measured today from `folio-designer/dist/offline-release-manifest.json`: 80 assets, 30 core / 50 deferred; `brotli.totalBytes` 14,589,414; `brotli.core` 29 / 6,398,032; `brotli.catalogue` 31 / 3,151,569.

**The assertions that must move together**
- `folio-designer/src/font-catalogue.test.ts` — `:461-481` **the block to retire** (`:467` family, `:468-475` Regular-only, `:476` no variable tables, `:478-479` glyf/`.ttf`); prose at `:35-40`; `:335` floor ≥31; `:337` family uniqueness; `:346` directory uniqueness; `:343` shipped population is 13; `:355-448` LICENSE/NOTICE/binary-identity/nameID-13 per row; `:451-458` the generated-count tie; `:484-522` the cmap↔`scripts` check; `:524-541` the no-descriptor emitter assertion; `:654-760` **the existing per-cut pattern to copy**, incl. `:736`.
- The **four interlocked population floors**, which move together by their own comments: `font-catalogue.test.ts:335`, `font-index.test.ts:154`, `font-name-table.test.ts:36`, `font-provenance.test.ts:62`.
- `folio-designer/src/font-index.ts:119` **`localByFamily = new Map(catalogueFaces.map(...))` — last-wins, no tie-break.** Silent today only because the catalogue is one Regular per family; with cuts it returns whichever row sorted last. One of four family-collapse sites in the designer; story 1 owns `font-source.ts:196-198` and `font-index.ts:257-260`, **this one is this story's**, and `:405` `indexByFamily` is the snapshot's own and unaffected.
- `folio-designer/src/font-index.ts:149` `addableFromTheWeb = (row) => !row.variable`; `:163` `webFamilies`; `:172` `addableFamilyCount = webFamilies.length + catalogueFaces.length` — **counts rows, so it triple-counts once a family has four**.
- `folio-designer/src/font-index.test.ts:189-197` offers a locally-held family **exactly once**; `:207` `toBe(10)`; `:267` the addable-count tie.
- `folio-designer/src/font-binary-identity.test.ts` — `it('gives every declared family a source file of its own')` and the `catalogueDeclaredFamilies()` join.
- `folio-designer/src/held-local-faces.ts:8`, `:61`, `:93` — probes one cache entry per catalogue face through a family-keyed Set.
- `folio-designer/e2e/font-embed-boundary.spec.ts:51-53`, `:266-272` — *"the release must carry one catalogue asset per catalogue family"*, computed from `families.length`; that premise breaks.
- `folio-designer/scripts/offline-release-contract.test.mjs:244-263` — builds `idOfFamily` as a family-keyed Map.
- `folio-designer/src/App.font-store.test.tsx:530-532` — `toHaveLength(catalogueFaces.length + 1)`.
- `folio-designer/src/canvas-font-stack.test.ts:426-434` — `toBe(14)` rule spellings.

**Cross-module source-text couplings (fragile, verify before editing)**
- `folio-go/internal/fontset/licencesignature_test.go:735`, `:856` read `font-catalogue.test.ts` **as source text** and fail if `const licenceSignatures`, `licenceSignatures[face.licence]` or `.toMatch(signature as RegExp)` disappear.
- `licencesignature.go:16-18`, `component_commands.go:4341-4342` and `folio-go/fonts/accounting_test.go:103` cite `font-catalogue.test.ts` **by line number** — any line shift makes them stale.
- `folio-go/fonts/fonts_test.go:89-92` and `folio-go/fonts/fonts.go:72-80` state the "catalogue stays Regular-only" rationale in prose; `component_commands.go:4413` says the same. All die with this story.
- `_bmad-output/specs/spec-fonts/font-catalogue.md:200` already reserves the field: *"`style` — the instance shipped (`Regular` initially; see the bold/italic open question)"*.

**Do not change**
- `folio-designer/src/shipped-face-cuts.ts:59-83` — the shape to follow (absent key = absent cut), not to edit.
- `tools/fontgen/instance_faces.py` — it is the ENGINE's pipeline and is not extended here, at all.
- `folio-designer/src/absent-face-recovery.ts` — fenced by story 1; add a test, do not edit it.
- `folio-designer/src/font-index.ts:183` `localByFamily` — story 1 already made it a face-set map.
- `folio-designer/public/templates/starter.folio`, the core tier, `CORE_CATALOGUE_FACE_IDS`.

## Tasks & Acceptance

**Execution:**
- [x] `folio-designer/public/fonts/` -- for each of the 78 cuts, fetch its family's pinned archive, verify the archive against the SHA-256 its NOTICE already records, extract the EXACT filename (see the prefix trap), and commit it as one directory per cut with exactly three files (the `.ttf`, one `LICENSE*`, one `NOTICE.md`) -- the tier's existing per-face provenance shape; `build-wasm.mjs:383-388` requires exactly one `LICENSE*` per directory.
- [x] `folio-designer/font-catalogue.json` -- add a `style` key to every existing row (`Regular`) and one row per new cut, keeping `family` as the BASE family (`Inter`) and letting `style` carry the cut -- 31 rows become 109; the CSS name is derived from the pair, never stored, so `font-catalogue.test.ts:467`'s binary-family claim stays true.
- [x] `folio-designer/scripts/build-wasm.mjs` -- add `style` to the `:296` required-field list with a closed-set check (`Regular|Bold|Italic|BoldItalic`); re-key the `:313` uniqueness guard (and its `:294` seed) to **(family, style)**; derive the `@font-face` family name per cut at `:584` without adding a descriptor; replace the `:460` `style: "Regular"` stamp with the row's value; rewrite the `:107-126` rationale and the stale 31-face/slot-count prose at `:335`, `:341`, `:374`, `:449-450`, `:595` -- the comment at `:110-112` asserts a rule this story ends.
- [x] `folio-designer/src/font-catalogue.test.ts` -- replace `:461-481` with a per-row claim that the binary's subfamily, `usWeightClass`, `fsSelection` bold/italic bits, `macStyle` and `italicAngle` match the declared `style`; keep `:476`/`:478-479`; update the `:35-40` prose; raise the `:335` floor -- an invariant is being retired, not deleted; `:654-760` is the working pattern.
- [x] `folio-designer/src/font-index.test.ts`, `font-name-table.test.ts`, `font-provenance.test.ts` -- raise the other three population floors in the same commit -- their own comments say all four move together.
- [x] `folio-designer/src/font-index.ts` -- make `addableFamilyCount` (`:247`) count distinct families rather than catalogue rows -- it is `webFamilies.length + catalogueFaces.length` today and would report four Inters. Leave `localByFamily` (`:183`) alone; story 1 already made it a face-set map.
- [x] `folio-designer/src/held-local-faces.ts` -- a family is `held` only when every cut it declares is cached, not when any one is (`:93-94`) -- otherwise a family with only its Regular cached reads as complete and CAP-4 never offers to finish it.
- [x] `folio-designer/src/absent-face-recovery.test.ts` -- add a case proving `absentFaceNames` still resolves a cut name against a candidate set of ~122 rather than 44, including a base/cut pair such as `Inter` vs `Inter Bold` -- the longest-first defence at `:73-88` is pre-existing and must be shown to hold at the new scale. Do NOT modify `absent-face-recovery.ts`.
- [x] `folio-designer/src/release-payload.ts` -- raise `maximumCacheAssets` (`:72`) 90 → 168 and `warnCacheAssets` (`:91`) 82 → 160, re-derived from the emitted manifest first, each still `^const <name> = <digits>$` on its own line, with a rationale comment beside it naming the measured count -- the reader at `offline-release-contract.mjs:204-208` is line-anchored and throws on ≠1 live match.
- [x] `folio-designer/scripts/generate-offline-release.mjs` -- `:237` `brotli.catalogue.familyCount` now counts faces, not families; rename it and move `verify-offline-release.mjs:409` with it -- a field that lies is worse than one that reds.
- [x] `folio-designer/e2e/font-embed-boundary.spec.ts`, `src/held-local-faces.ts`, `scripts/offline-release-contract.test.mjs`, `src/App.font-store.test.tsx`, `src/font-binary-identity.test.ts` -- repair the family↔asset 1:1 premise in each -- measured: each assumes one catalogue face per family.
- [x] `folio-go/fonts/fonts_test.go:89-92`, `folio-go/fonts/fonts.go:72-80`, `folio-go/component_commands.go:4413` -- rewrite the "catalogue stays Regular-only" rationale -- it is now false; leave the tests' behaviour alone.
- [x] Re-check `licencesignature_test.go:735,:856` still find their three source-text needles, and refresh the line-number citations at `licencesignature.go:16-18`, `component_commands.go:4341-4342`, `fonts/accounting_test.go:103` -- they read `font-catalogue.test.ts` by line.

- [x] **F2** `folio-designer/src/App.tsx` -- teach `cutEmbedPlan` (`:5608`) and `commitPropertiesEmbeddingCuts` (`:3083`) to resolve a LOCAL-tier cut from `catalogueFaces` and read its bytes from the release cache via `fetch(cut.url)`, building the embed record from the catalogue row's own family/style/licence/licenceText/copyright/source -- the face store is for fetched web faces only; a second copy of a committed face there is refused. A cut whose bytes are not cached is refused BY NAME with a remedy, never silently fetched from upstream.
- [x] **F2** `folio-designer/src/App.tsx` -- `cutAbsenceState` (`:5721`) must answer all four states for a local family, treating the shipped catalogue as that tier's census -- it returns `unpublished` today for a cut the release ships, which is the sentence CAP-2 forbids. Rewrite the `settled fork 4` comment at `:5714-5720`, which states the retired Regular-only premise as its reason.
- [x] **F1** `folio-designer/src/font-index.ts` -- `familyIsInstalled`'s local arm (`:434`) must answer "can these bytes be used" and `familyIsComplete`'s (`:478`) "is anything left to fetch"; they are the identical expression today. Make the code satisfy the doc comment already at `:452-460`. Cover the no-install path: `browserSpecimenBytes` caching each local row's Regular while browsing must not drop that family out of AVAILABLE LOCALLY.
- [x] **F3** `folio-designer/src/font-browser-model.ts` -- correct `weightLine` (`:380-383`); confirming now installs up to four faces per staged local family. Re-point `font-browser-model.test.ts:245,246,276` and `e2e/font-browser.spec.ts:153` at what is now true. Story 5's pre-pick CUT DISPLAY stays untouched.
- [x] **Core byte warning** (owner-authorised) `folio-designer/src/release-payload.ts` + `scripts/` -- add an approach warning for `maximumCoreCacheBytes` in the idiom `warnCacheAssets` already uses: its own line-anchored `const`, derived by the text reader, warning with the remaining margin. The core tier is at 97.8% and its first signal today is a hard build failure.
- [x] **Patches F4-F15** -- `licencecensus_test.go:22-26` hand-typed count (82/73 -> the real 160/151) and move the "FIRST Apache-2.0" comment above its row at `:240`; retire the Regular-only rationale at `font-binary-identity.test.ts:1157-1158` and `canvas-font-stack.test.ts:840-841`; correct `build-wasm.mjs:324-328`'s claim that the style spellings match `font-source.ts` (`BoldItalic` vs `Bold Italic`) and `:133`'s citation of `shipped-face-cuts.ts`; add a test tying each cut's archive URL and digest to its family's Regular ("both come from the same pinned archive" is claimed 76 times and checked nowhere); correct `font-catalogue.test.ts:452-455`'s overclaim that the sha256 tie catches a repointed row, and `:587`'s "other 22" (the sloped population is 46, of which 44 lack the OBLIQUE bit); refresh the stale "31 catalogue faces" prose at `font-index.ts:400`, `App.tsx:691`, `App.tsx:2738`, `e2e/font-embed-boundary.spec.ts:223`, `offline-release-contract.mjs:91` and the "one day carries more than one cut" comment at `App.tsx:2769-2771`; fix `accounting_test.go:103-104`'s citation; "It is a upright" -> "an upright" in the 30 Bold NOTICEs; parallelise the e2e priming loop at `font-embed-boundary.spec.ts:274-278`.

**Acceptance Criteria:**
- Given a committed family with a bold and no network, when the author presses **B**, then the bold is embedded from the release cache and paints with NO diagnostic, and the document declares the cut.
- Given a committed family whose Regular alone is cached, when the family control is opened, then that family is still offered under AVAILABLE LOCALLY and is usable, while the font browser still offers it for completion.
- Given a staged local family in the Add font dialog, when the footer states what will be installed, then the sentence is true of the faces confirming actually fetches.
- Given a core tier approaching its byte ceiling, when the release is verified, then a warning names the remaining margin before a build failure is the first signal.
- Given the 31 committed families, when the catalogue is rebuilt, then every cut that family publishes is declared with its own `style`, and a family publishing no italic declares none.
- Given a rebuilt release, when `npm run build` runs, then `verify:offline` passes, the core tier is still exactly 30 assets, `maximumCoreCacheBytes` is not exceeded, and the emitted asset count is at or below the newly derived `maximumCacheAssets`.
- Given any catalogue row, when `font-catalogue.test.ts` runs, then the binary's own subfamily and weight class are asserted against that row's declared `style`, and a row mislabelled `Bold` over a Regular binary reds naming the row.
- Given the release manifest, when the tiers are classified, then every new cut is `deferred`, `CORE_CATALOGUE_FACE_IDS` still resolves to `catalogue-roboto` alone, and the core tier's Brotli weight is still under `maximumCoreCacheBytes` despite the larger inlined licence/provenance text in `font-catalogue.ts`.
- Given four cuts of one family in the catalogue, when the Add font dialog lists families, then that family appears exactly once and each cut resolves to the row whose `style` matches — never to whichever row sorted last.
- Given an author offline on a committed family with a bold, when they press **B**, then the bold paints with no diagnostic.
- Given Roboto, when its cuts are resolved, then they come from the existing core assets and the catalogue declares no Roboto cut — and the code that joins the two says so in a comment.

## Implementation Notes

### DM SANS SHIPS TWO CUTS, AND THE ABSENCE IS A REFUSAL — NOT AN UPSTREAM GAP

This is the one place the acquisition departs from the frozen Intent's "78 new files / 158 assets".
**The real figures are 76 new files, 107 catalogue rows, 156 release assets.** Owner-ruled on
2026-09-20 (option D first, falling back to option B); the frozen block predates that ruling and is
left unedited, as only a human reopens it.

**What is wrong at `4412393b`.** `Sans/fonts/ttf/DMSans-Italic.ttf` cannot prove its own style:

    name[1]='DM Sans Italic'  name[2]='Regular'   name[16]='DM Sans'  name[17]='Italic'
    usWeightClass=400  fsSelection=0x00c0  macStyle=0x0  italicAngle=-10.0

Two separate things, and only one is a defect. The **naming is a legitimate convention** — a family
with more than four styles pushes the extras into their own RIBBI family, which is why name[1]/[2]
read `DM Sans Italic`/`Regular` while the typographic name[16]/[17] read `DM Sans`/`Italic`. The
**bits are simply wrong**: `fsSelection`'s ITALIC bit is clear and `macStyle` is `0x0`, both claiming
upright, while `italicAngle` is -10 — the outlines *are* italic. Its own sibling
`DMSans-BoldItalic.ttf` is correct (`name[1]='DM Sans'`, `name[2]='Bold Italic'`,
`fsSelection=0x00a1`, `macStyle=0x3`), which is what makes this a defect rather than a house style.

**What was searched (option D).** Every commit that has ever touched that path — `239ca0b79c`,
`c261d25928`, `1c3bad965d`, `7c79670d40`, `d0520ba03b`, `4412393b7d` — carries the **same** wrong
bits; the current pin is the newest of the six, so the defect is not a regression and there is no
forward fix. The repository's only two releases/tags (`v1.002`, `v5.003`, both @ `a9ec4224`, 2019)
ship `DeepMindSans`/`DeepMindSerif` and contain no `DMSans*.ttf` at all. The `google/fonts` mirror is
excluded by the tier's own rule — the bytes come from the project, never the mirror. Worth recording
because it was checked: the committed Regular matches `4412393b` **exactly** and differs at all five
older commits, so a backward re-pin would also have changed the Regular's bytes.

**Conclusion.** Option D is exhausted, so option B stands: **DM Sans ships Regular and Bold only.**
The italic *exists upstream but cannot prove its own style*, so its absence here is a **deliberate
refusal, not an upstream gap** — which is the distinction CAP-2's sentence rests on, and the answer
to anyone later wondering why DM Sans is short. `DMSans-BoldItalic.ttf` is withheld with it rather
than shipping option A's bold-italic-without-italic shape. **The guard stays absolute: no named
exception, no allowlist, no "known upstream bug" clause.**

### Acquisition record

All 27 pinned archives were downloaded and verified against **both** the SHA-256 and the byte length
their NOTICEs record — zero mismatches. Matching was by **exact filename**, and the inverse check
passed too: every cut the inventory called absent is genuinely absent from its archive. The sfnt
reader used to judge the faces was positive-controlled against the **31 committed Regulars first**
(0 disagreements), so "one face failed" is a finding about that face and not about the reader.

Of the 76 acquired faces: **76/76** agree with their declared style on all five fields
(`subfamily`, `usWeightClass`, `fsSelection`, `macStyle`, `italicAngle`); **76/76** have `name[1]`
equal to the base family and `name[2]` equal to the style label, so `font-catalogue.test.ts:467`
holds unchanged with `family` carrying the base name; **76/76** have a nameID 13 matching their
declared SPDX signature and a non-empty nameID 0. New raw bytes: **21,757,008 (20.75 MiB)**.

The three Adobe families' slope spelling was recorded as upstream ships it — `-It` / `-BoldIt` —
not normalised.


### Measured outcome

| | before | after |
|---|---|---|
| Catalogue rows | 31 | **107** (31 families; Roboto 1 row, DM Sans 2, six italic-less 2, the rest 4) |
| New committed faces | — | **76** (+21,757,008 raw bytes, 20.75 MiB) |
| Release assets | 80 (30 core / 50 deferred) | **156** (30 core / **126** deferred) |
| `maximumCacheAssets` | 90 | **166** — 156 measured + the same 10-slot reserve the 90 carried over 80 |
| `warnCacheAssets` | 82 | **158** — eight below the ceiling, the existing one-batch margin |
| Core tier assets | 30 | **30** — pin untouched, every new face `deferred` |
| Core Brotli bytes | 6,392,910 | **6,407,027** of 6,553,600 → **146,573 headroom** |
| `brotli.catalogue` | `familyCount` 31 | `faceCount` **107** / 11,517,610 |

The budget pair is 166/158 rather than the ruling's literal 168/160 **because the ruling was the
arithmetic, not the literal**: it was stated over 158 assets, and the DM Sans refusal (ruled after
it) makes the real figure 156. Both were re-derived from the emitted manifest, not typed.

The core bundle was measured, not assumed: 76 more inlined licence texts cost **14,117 Brotli
bytes** against 146,573 of remaining headroom, so **no licence-text deduplication was needed and the
ceiling was not moved**.

### Verification run

- `npm run build` — EXIT 0. 156 assets, 30 core / 126 deferred; `brotli.core` 29 / 6,407,027.
- `npm run typecheck` — EXIT 0. `npm run lint` — EXIT 0, **exactly 8** `only-export-components`, the baseline.
- `npm test` — EXIT 0, **89 files, 2093 passed, 0 failed**.
- `cd folio-go && go test ./...` — **exactly one failure**, `TestCorpusMeetsP6ExerciseFloors/P6g`
  (`got 7, need >=20`), the mandated pre-existing red. Nothing else.
- `cd lint && go test -count=1 ./...` — 4 packages `ok`. `-count=1` used deliberately: this module
  walks the `folio-go` tree, and Go's test cache does not track `ReadDir`, so a cached `ok` here is
  no measurement at all.
- **Test name-set diff** (not counts): **1 removed** — `ships every catalogue face as a single
  upright static Regular…`, the sanctioned retirement — and **5 added**: the per-row style claim,
  the four-cut discrimination proof, the short-family record, the ~120-candidate
  `absentFaceNames` proof, and the held-family cache test. No Go test functions added or removed.
- **The new guard was mutation-proven, not merely observed green.** Relabelling `arimobold` as a
  `Regular` row over Bold bytes reds it with: *"font-catalogue.json declares it the 'Regular' cut of
  'Arimo' and the bytes say family 'Arimo', subfamily 'Bold', usWeightClass 700"*. Restored after.

### Out-of-fence edit reverted

The implementation subagent also amended `_bmad-output/specs/spec-fonts/font-catalogue.md` (+6/-2),
correctly describing the new `style` field and the derived CSS name. **I reverted it**: `spec-fonts/*`
is another story's artifact and outside this story's fence. The edit was right and is worth making —
it is recommended to whoever owns that file, not absorbed here. `spec-deferred-offline-cache/asset-tiers.md`
WAS kept: `release-payload.ts` names it as the measurement to re-take whenever the tiers move, so it
is this change's own record rather than another story's.

### Loopback 1 — verification record

Re-verified after F1/F2/F3, the core-byte warning and the twelve patches:

- `npm run build` — **EXIT 0**. 156 assets, 30 core / 126 deferred; `brotli.core` 29 / **6,407,803**
  (145,797 under the 6,553,600 ceiling, **14,725 under the new 6,422,528 warning — silent, as
  designed**); `brotli.catalogue` 107 / 11,517,610. All seven budget constants are exactly one live
  `^const <name> = <digits>$` line each.
- `npm run typecheck` — EXIT 0. `npm run lint` — **exactly 8** `only-export-components`, the baseline.
- `npm test` — **EXIT 0, 89 files, 2108 passed, 0 failed.**
- `cd folio-go && go test ./...` — exactly one failure, `TestCorpusMeetsP6ExerciseFloors/P6g`
  (`got 7, need >=20`), the mandated pre-existing red.
- `cd lint && go test -count=1 ./...` — 4 packages `ok`.
- `npx playwright test e2e/font-embed-boundary.spec.ts e2e/font-browser.spec.ts` — **9 passed (4.0m)**.

**F2 was red-proven independently, not taken on trust.** Disabling the catalogue fall-through in
`catalogueCutOf` (`App.tsx:5641`) — i.e. restoring pre-fix behaviour — turns all five new embed tests
red together: *embeds the bold from the release cache*, *embeds the BOLD ITALIC cut across the two
style spellings*, *keeps the original sentence for a committed family that publishes no italic*,
*refuses by name when the bundled cut cannot be read*, and *writes no copy of the committed face into
the machine store*. Restored and re-run green.

**Two defects found during this round and fixed rather than absorbed.** (1) Three new comments in
`App.tsx` spelled `IndexedDB`, which `file/file-access-contract.test.ts:91`'s raw-text prohibition
flags for any non-exempt module — zero such spellings existed at the parent commit. Reworded to "the
machine face store", which is the encapsulation the guard exists to protect; the guard was not
touched. (2) The new *"is not already firing on the release this repository emits"* test read
`dist/offline-release-manifest.json`, making it **the only test in the default suite depending on an
artifact that suite never builds** — `npm test` is `build:wasm && vitest run`. It greened on a dirty
tree and redded on a clean one. Removed, with its claim relocated to where the manifest always
exists: `verifyOfflineRelease` calls `reportCoreCacheByteApproach` on the real release
(`verify-offline-release.mjs:470`) inside `npm run build`, so an over-threshold core tier warns on
every build with its margin. The mechanism stays covered by three tests that do run.

**Two implementer judgement calls, reviewed and accepted.** `readHeldLocalFamilies` was replaced by
`readLocalFaceHoldings` returning `{ usable, complete }` — the KEEP list protected the all-cuts
RULE, not the function name, and the rule is preserved verbatim and still asserted directly
(`held-local-faces.test.ts:75`). Taking a record instead of two interchangeable bare `Set`s makes the
F1 fusion impossible to reintroduce at the type level, which is stronger than what the spec asked
for. The three cross-language citations were converted to SYMBOL NAMES rather than re-derived line
numbers; those numbers had already drifted twice, and a symbol survives a line shift.

**Environment note, not a code defect.** Several `npm run build` attempts failed spuriously with
`ENOTEMPTY`/`ENOENT` on `dist/assets`. A `vite` dev server had been running for 31 hours and
concurrent builds were emptying a `dist` another build's Brotli step was still writing into. One
foreground build with a >4-minute budget is clean.

## Spec Change Log

- **2026-09-20 — loopback 1, from review finding F2 (intent_gap) and F1/F3 (bad_spec).**
  - *Triggering findings.* F2: a committed family's cut could never be embedded — `cutEmbedPlan`
    resolves only from `storedFaces`, the local install writes only to the release cache, and
    `cutAbsenceState` answered `unpublished` for a bold this release ships, so the story's own AC and
    CAP-3 were both false. F1: `familyIsInstalled`'s local arm silently became a completeness answer
    when `readHeldLocalFamilies` changed meaning, dropping a partially-cached family out of AVAILABLE
    LOCALLY. F3: the dialog still said *"one upright Regular each, no bold or italic"* while a staged
    local family installs up to four faces.
  - *What was amended.* The frozen `Never` fence was moved by the owner to admit the embed path (the
    only edit to the frozen block, attributed inline). Three decisions added: the catalogue is the
    resolution source for a local cut, its bytes come from the release cache and never the network,
    and the shipped catalogue is the local tier's census. Tasks added for F1, F2, F3, the twelve
    patch-grade findings, and an owner-authorised approach warning for the core BYTE ceiling.
  - *Known-bad state avoided.* Re-deriving without this would rebuild a catalogue whose 76 faces
    paint on canvas but can never enter a document, under a panel that denies they exist — the data
    half of CAP-3 with none of its user-visible half.
  - *KEEP — what worked and must survive re-derivation.* The 76 committed faces and their provenance
    records are VERIFIED and must not be re-acquired: archives matched both SHA-256 and byte length,
    76/76 proved their style from their own bytes, 76/76 matched base family and style label, 76/76
    matched their SPDX signature. The per-row style guard is mutation-proven against four separate
    mutations and must not be weakened or given an exception. DM Sans stays at Regular + Bold as a
    recorded refusal. `maximumCacheAssets`/`warnCacheAssets` stay DERIVED from a real manifest.
    `readHeldLocalFamilies`' all-cuts rule is CORRECT — F1 is about its reader, not about it.
    `absent-face-recovery.ts` stays unmodified. The 30/30 core pin and `CORE_CATALOGUE_FACE_IDS`
    stay untouched.


**2026-09-20 — loopback 1 implemented.** F1, F2, F3, the core-byte approach warning and the
twelve patches are done, and three shapes are worth recording because they were choices rather
than transcriptions.

**F2 — the embed path.** `CutEmbedPlan` became a discriminated union over the two tiers:
`cutEmbedPlan` asks the IndexedDB store first and falls through to `catalogueFaces`, and
`commitPropertiesEmbeddingCuts` reads a local plan's bytes with `fetch(cut.url)` AFTER a
`localFaceIsHeld` probe of the release cache — a cut the cache has lost is refused by name with
a remedy rather than fetched. Nothing is written to the store. Two vocabularies had to be joined
explicitly and are now named in one place (`CATALOGUE_CUT_STYLES`): the catalogue spells the
combined cut `BoldItalic` (the `.folio` variant key) and the store spells it `Bold Italic` (the
RIBBI subfamily), so the lookup uses the catalogue's and the DOCUMENT is written in the store's,
which keeps one vocabulary in a `.folio` whichever tier served the bytes. `cutAbsenceState` now
consults the catalogue as the local tier's census before it reaches the store's, so a committed
family answers `unpublished` only when the catalogue declares no such cut — true of the six
italic-less families and of DM Sans's withheld italic, and false of everything else it used to
say it about. All five new tests were RED-PROVEN against the unfixed `cutEmbedPlan`.

**F1 — one probe, two answers.** `familyIsInstalled` and `familyIsComplete` could not be
separated while both took a bare `ReadonlySet<string>`: that interchangeability is how the fusion
survived review once. `readLocalFaceHoldings` now returns `{ usable, complete }` from ONE pass
over the cache — `usable` is "this family's Regular is cached", `complete` is the ALL-CUTS RULE
UNCHANGED — and both predicates take that record and read their own field, so passing the wrong
one no longer type-checks. `readHeldLocalFamilies` and `initialHeldLocalFamilies` were removed
rather than left as projections nothing but their own tests would call; the all-cuts rule they
carried is asserted directly on `readLocalFaceHoldings().complete`.

**F3 — the weight line.** `weightLine` now reads `N families · every cut each one publishes, up
to four`. The noun moved with the fact: `staged` has always counted families, and "faces" was
only the same number under the retired one-face rule. The existing bans (no subsetting language,
no destination language) still hold and are still asserted at every staged count.

**The core byte warning** is `warnCoreCacheBytes = 6422528`, the ceiling less 128 KiB, read by
`declaredCoreCacheByteWarning` and emitted by `reportCoreCacheByteApproach` on the same
`reportApproach` option the count warning uses. It is deliberately several batches wide rather
than one: today's margin under the CEILING is 145,797 bytes and this story cost ~14,900, so the
build is silent on the commit that declares the threshold, the next comparable batch crosses it
and says so, and the ceiling itself is about nine batches further on. A threshold one batch below
the ceiling would be crossed and breached by the same commit.

**Two citations were converted from line numbers to symbol names** rather than re-derived
(`fonts/accounting_test.go`, `internal/fontset/licencesignature.go`,
`component_commands.go`). Both had already drifted to unrelated code before this story moved
those files again; a line citation across languages has no gate behind it and rots on the next
edit either side.

**2026-09-20 — implemented.** The two budget constants were DERIVED from a real
`npm run build` rather than taken from the frozen Intent's proposal, and they came out two
below it: `maximumCacheAssets` 90 → **166** and `warnCacheAssets` 82 → **158**, over a measured
**156** emitted assets (30 core / 126 deferred). The frozen block proposed 168/160 over 158
assets; that figure predates the owner's DM Sans ruling recorded in Implementation Notes, which
withholds `DMSans-Italic.ttf` and `DMSans-BoldItalic.ttf`. 76 new faces, 107 catalogue rows,
156 assets — the Implementation Notes figures, confirmed by the build. The reserve (10 slots)
and the warning margin (8 below the ceiling) are the ones this file already used, unchanged.

**Core byte ceiling: measured, not breached.** `maximumCoreCacheBytes` stays 6,553,600. The
enlarged `src/generated/font-catalogue.ts` — a ~4 KB licence text, a copyright and a source
string per face, bundled core-tier — took the core tier from 6,392,910 to **6,407,803** Brotli
bytes: **145,797 bytes of headroom**, down from 160,690. No licence-text deduplication was
needed. `brotli.core.assetCount` is 29 and `coreAssetCount` is 30, both unmoved.

## Review Triage Log

Four layers ran: blind-hunter, edge-case-hunter, verification-gap, plus a targeted layer aimed at the
five areas the coordinator named. None skipped. Every finding verified at its cited location before a
verdict was rendered.

| # | verdict | finding and evidence | route |
|---|---|---|---|
| F1 | **high** | `familyIsInstalled`'s local arm silently became a COMPLETENESS answer. `font-index.ts:434` and `:478` are now the identical expression `heldLocalFamilies.has(source.family)`, because this story changed what that set means. `familyIsComplete`'s own doc comment (`:452-460`) states the rule being broken: *"A family holding only the Regular of a family that publishes a Bold answers YES to the first and NO to the second"*. It now answers NO to both, so `App.tsx:6176` drops a partially-cached committed family out of AVAILABLE LOCALLY entirely — D-4's *"a font already sitting on the machine UNUSABLE"* outcome. Reachable without installing anything: `browserSpecimenBytes` (`App.tsx:1102-1114`) caches only each local row's Regular while browsing. | bad_spec |
| F2 | **high** | **A committed family's cut can never be embedded, and the panel denies it exists.** `cutEmbedPlan` (`App.tsx:5608`) resolves a cut only from `storedFaces`; `installFamily`'s local arm (`App.tsx:2769-2784`) fetches cut URLs into the RELEASE CACHE and deliberately not the face store (`:2754-2756`: *"a second copy there would be two answers to one question"*). So no plan is ever built for a catalogue cut, and `cutAbsenceState` (`App.tsx:5721`) returns `'unpublished'` for a local family with no census — *"No bold face in this family"* about a bold this release now ships. This story's own AC *"press **B**, then the bold paints with no diagnostic"* does not hold, and CAP-3's *"No offline author loses bold that an online author would get"* is false. | **intent_gap** |
| F3 | medium | `weightLine` (`font-browser-model.ts:380-383`), rendered at `FontBrowser.tsx:366`, still tells the author `N faces · one upright Regular each, no bold or italic` at the moment they confirm — while staging one local family now installs up to four faces via `App.tsx:2772-2779`. Three tests pin the false string (`font-browser-model.test.ts:245,246,276`, `e2e/font-browser.spec.ts:153`), so the guard is green over it. Kept rather than scoped out: the spec's Never fenced the dialog to story 5, but the INTENT does not. | bad_spec |
| F4 | medium | `lint/internal/licence/licencecensus_test.go:22-26` still reads *"82 rows: 73 committed … plus the 9 dependency"*; measured, the table now holds 160 rows / 151 committed. The same paragraph warns this is *"the ONLY place either is typed by hand"* and records two prior instances of identical rot. This change caused the third. | patch |
| F5 | medium | Two comments still give the RETIRED Regular-only rule as the REASON for the Roboto split: `font-binary-identity.test.ts:1157-1158` and `canvas-font-stack.test.ts:840-841`, both *"a bold cut cannot be a catalogue face at all"*. `fonts_test.go` was corrected; these were missed. A reader trusting them concludes the split is mechanical rather than a ruling about the 30/30 core pin, and "fixes" it by moving Roboto's cuts into the catalogue. | patch |
| F6 | medium | `CATALOGUE_STYLES`' comment (`build-wasm.mjs:324-328`) claims the spellings are *"the ones `src/font-source.ts` stamps on a fetched face"*. False for the combined cut: the catalogue spells `BoldItalic`, `font-source.ts:229-233` spells `Bold Italic`. Nothing compares them today, but the comment invites a direct comparison that would silently never match. | patch |
| F7 | medium | Every new NOTICE claims *"both come from the same pinned archive"* and NOTHING checks it. Verified true by hand for all 31 families today (29 distinct archives, no cross-project digest reuse), but a later batch could assemble a family from two upstream releases and stay green. | patch |
| F8 | medium | The NOTICE sha256/size tie (`font-catalogue.test.ts:452-456`) presents itself as the catch-all for *"the binary was swapped — a different weight, a different style"*, but it travels WITH the directory, so it cannot see a row repointed at another family's file. Proven: repointing `interbold` at `notosans-bold/NotoSans-Bold.ttf` leaves `:456` and `:463` passing; only the name-table family check at `:565` fires. The comment overclaims. | patch |
| F9 | low | Stale "31 catalogue faces" prose at `font-index.ts:400`, `App.tsx:691`, `App.tsx:2738`, `e2e/font-embed-boundary.spec.ts:223`, `offline-release-contract.mjs:91` (106 are deferred now), and `App.tsx:2769-2771` still calls this story's own path hypothetical: *"WHEN THE CATALOGUE ONE DAY CARRIES MORE THAN ONE CUT"*. | patch |
| F10 | low | `build-wasm.mjs:133` cites `src/shipped-face-cuts.ts` as carrying the Roboto seam statement; that file is untouched by this change and carries no such statement. | patch |
| F11 | low | 30 Bold NOTICEs read *"It is a upright **Bold** static cut"* — should be "an upright". The sloped variants are correct. | patch |
| F12 | low | `font-catalogue.test.ts:587` says the OBLIQUE bit *"asserting it set on every italic would be false of the other 22"*. `cut.italic` covers Italic and BoldItalic — 46 sloped faces, of which 2 set the bit and 44 do not. | patch |
| F13 | low | `licencecensus_test.go:240` inserts `robotoslab-bold/LICENSE-APACHE.txt` immediately above the comment that reads *"THE FIRST Apache-2.0 FONT ASSET IN THE REPOSITORY"*, which now sits between the two Apache rows and describes the second. Sorted placement is right; the comment must move. | patch |
| F14 | low | `folio-go/fonts/accounting_test.go:103-104` cites `e2e/font-embed-boundary.spec.ts:151-158` as a TS-side precedent for exactly-one extraction; `readBoundarySentences` throws on zero and on fewer than five, never on two-or-more. | patch |
| F15 | low | `e2e/font-embed-boundary.spec.ts:274-278` primes the catalogue with one sequential `await fetch` per asset — 31 round-trips became 107, twice per run. It passed at 3.8m; a concurrency pool keeps it off the timeout budget. | patch |
| F16 | **false** | *"`addableFamilyCount` now counts rows rather than families."* Checked: `font-index.ts:263` is `webFamilies.length + localFamilies.length`, `localFamilies` is one entry per family, and `font-index.test.ts:283` re-derives it independently as `webFamilies.length + new Set(catalogueFaces.map(f => f.family)).size`. Every consumer is consistent. | — |
| F17 | **false** | *"`readHeldLocalFamilies` still reports a family held when only some faces are cached."* Checked: `held-local-faces.ts:122-128` counts per family and admits only on `count === declaredCutsPerFamily.get(family)`, and `held-local-faces.test.ts:63-68` plants a partial family and asserts it is NOT held. The predicate is right; F1 is that its READER changed meaning with it. | — |
| F18 | **false** | *"The new per-row style guard misses a different-weight or wrong-slope swap."* All three unchosen mutations were run against the real suite and caught: a synthesised Inter Medium under a Bold row, BoldItalic bytes under an Italic row, and a directory repointed at another family — first failure `font-catalogue.test.ts:566`, `:566`, `:565` respectively. Tree restored. | — |
| F19 | medium, unverified | Seven Regular NOTICEs (`cascadiacode`, `cascadiamono`, `dmsans`, `oswald`, `sourcecodepro`, `sourcesans3`, `spacegrotesk`) record a LICENSE sha256/size wrong by 93 bytes — CRLF digests left behind when rename commit `d738941` normalised the files to LF. The NEW cut NOTICEs beside them record the correct LF digest, so two NOTICEs now state different digests for byte-identical files. Nothing checks either value. PRE-EXISTING, surfaced by this change. | defer |
| F20 | medium, unverified | There is no approach warning for the core BYTE ceiling analogous to `warnCacheAssets`. `maximumCoreCacheBytes` now sits at 97.8% (146,573 bytes of headroom), so the next batch's first signal is a hard build failure. Pre-existing absence. | defer |
| F21 | **rejected** | The spec's own unfrozen Design Notes still carry pre-ruling figures (112/78/109/158). Rejected on the standing rule that a finding whose fix is to edit this build's spec is not a review finding. The Implementation Notes carry the authoritative 107/76/156, and the Design Notes' provenance table is a record of what was measured BEFORE the DM Sans ruling. | — |

**Cascade.** F2 is an `intent_gap`, so it outranks everything below it and the patch entries are moot
until it is resolved — code will be re-derived. `review_loop_iteration` incremented to 1. Per the
standing instruction not to patch around a gap, the loop is HELD at the human rather than reverted:
reverting 259 files and a commit is destructive, and the owner may rule that the consumption path
belongs to a later story, in which case the data half of CAP-3 stands as committed.


## Design Notes

**The inventory is VERIFIED against the archives themselves, not against the Google snapshot.**
Every one of the 27 distinct upstream archives the 31 NOTICEs pin was read directly on 2026-09-20:
the 20 release ZIPs by HTTP-range-reading their central directories (each archive's served byte
length matched the size its NOTICE pins, exactly), and the 7 GitHub source tarballs through the
tree/contents API at the pinned commit or tag. **Positive control: all 31 committed Regulars have
byte sizes identical to their upstream originals**, which proves the path and naming resolution is
right for every family, not just the ones that were easy.

Result: **112 cuts, unchanged from the snapshot figure — but now measured.** The two families the
snapshot could not speak for are confirmed rather than assumed: **Inter Display** ships
`InterDisplay-Bold/Italic/BoldItalic.ttf` in `extras/ttf/`, and **Source Serif 4 Display** ships
`SourceSerif4Display-Bold/It/BoldIt.ttf` in `TTF/`. The six italic-less families are confirmed from
the archives too: Fira Code (Bold, Light, Medium, Retina, SemiBold — no slope at all), Noto Sans
Thai Looped, Noto Serif Thai, Oswald, Roboto Slab, Space Grotesk.

Per-family new-file byte totals (all 78, measured from the archive directories):

| Family | cuts | new | bytes | Family | cuts | new | bytes |
|---|---|---|---|---|---|---|---|
| Arimo | 4 | 3 | 1,496,712 | Open Sans | 4 | 3 | 453,828 |
| Cascadia Code | 4 | 3 | 1,516,512 | Oswald | 2 | 1 | 108,496 |
| Cascadia Mono | 4 | 3 | 1,456,596 | Plus Jakarta Sans | 4 | 3 | 403,064 |
| Cousine | 4 | 3 | 913,220 | Roboto | 4 | **0** | 0 |
| DM Sans | 4 | 3 | 244,528 | Roboto Condensed | 4 | 3 | 1,108,936 |
| Fira Code | 2 | 1 | 319,368 | Roboto Mono | 4 | 3 | 401,184 |
| Geist | 4 | 3 | 387,708 | Roboto Slab | 2 | 1 | 176,068 |
| Geist Mono | 4 | 3 | 462,016 | Source Code Pro | 4 | 3 | 542,492 |
| Intel One Mono | 4 | 3 | 381,268 | Source Sans 3 | 4 | 3 | 1,062,688 |
| Inter | 4 | 3 | 1,263,112 | Source Serif 4 Display | 4 | 3 | 701,628 |
| Inter Display | 4 | 3 | 1,257,872 | Space Grotesk | 2 | 1 | 116,056 |
| JetBrains Mono | 4 | 3 | 834,500 | Ubuntu Sans | 4 | 3 | 1,136,840 |
| Literata | 4 | 3 | 954,296 | Ubuntu Sans Mono | 4 | 3 | 654,676 |
| Lora | 4 | 3 | 639,112 | Noto Sans Thai Looped | 2 | 1 | 30,492 |
| Montserrat | 4 | 3 | 1,364,788 | Noto Serif Thai | 2 | 1 | 26,524 |
| Noto Serif | 4 | 3 | 1,508,600 | **TOTAL** | **112** | **78** | **21,923,180** |

**Measured figures (all from the working tree, not HEAD).**

| Quantity | Today | After |
|---|---|---|
| Catalogue rows | 31 | **109** (112 cuts less Roboto's 3, already shipped) |
| New committed faces | — | **78** |
| Release assets | 80 (30 core / 50 deferred) | **158** (30 core / 128 deferred) |
| Catalogue raw bytes | 8,466,552 (8.07 MiB) | 30,389,732 (**+21,923,180 = +20.91 MiB**, measured) |
| Catalogue brotli | 3,151,569 | ~11.6 MiB (est. at today's 37% ratio) |
| Release brotli total | 14,589,414 | ~23 MiB (est.; no ceiling reads this) |
| Core assets | 30 | unchanged |
| Core Brotli bytes | 6,398,032 (ceiling 6,553,600 — **155,568 headroom**) | **must be re-measured**: `font-catalogue.ts` inlines a ~4 KB licence text, a copyright and a source string per face, and that module is core-tier bundle |

**Why per-cut CSS family names rather than descriptors, and why the ROW still says `Inter`.**
`build-wasm.mjs:555-563` states the rule and `font-catalogue.test.ts:533-534` asserts it: no
`font-weight`, no `font-style`, because a weight axis in CSS is one the document format excludes.
The thirteen shipped rules already carry four Roboto and four Noto Sans cuts under that rule, as
separate CSS families. Adding descriptors instead would change four things and introduce a silent
last-rule-wins collapse at `:614-624` that the `:615` count check cannot see.

But the CSS name and the row's `family` are **not** the same string. A bold cut's binary reports
name[1] = the base family and name[2] = the cut, which `build-wasm.mjs:567-571` states and
`font-catalogue.test.ts:467` asserts. So the row stays `{ family: "Inter", style: "Bold" }` and the
CSS name is derived — which also keeps `localByFamily`, `addableFamilyCount`, the font browser and
CAP-5's "one entry per family" reading the base family, as they already do. The cost is that the
uniqueness key at `build-wasm.mjs:313` / `font-catalogue.test.ts:337` must move from `family` to
`(family, style)`.

**The one thing measurement could still overturn.** `src/generated/font-catalogue.ts` inlines a
licence text per face into the **core** bundle, against ~155 KB of Brotli headroom. If 78 more
faces breach `maximumCoreCacheBytes`, the fix is to deduplicate the licence texts by SPDX id —
`build-wasm.mjs:341-342` already observes that 31 faces emit 31 texts over only three identifiers —
rather than to move the ceiling, which guards the first-load screen.

## Verification

**Commands:**
- `cd folio-designer && npm run build` -- expected: `build:wasm`, `tsc -b`, `vite build`,
  `build:offline` and `verify:offline` all succeed; the emitted manifest's `assets.length` is at or
  below the new `maximumCacheAssets`, `brotli.core.assetCount` is 29 and `coreAssetCount` is 30.
  **Print the emitted asset count AND `brotli.core.totalBytes` and quote both in the report** —
  the first is what the 168/160 pair must be re-derived from; the second is the ~155 KB-headroom core
  byte ceiling the enlarged `font-catalogue.ts` bundle lands on.
- `cd folio-designer && npm run typecheck && npm run lint && npm test` -- expected: green; 4
  pre-existing `only-export-components` lint warnings (the baseline is exactly 8) are not a regression. Report pass counts.
- `cd folio-go && go test ./...` -- expected: exactly one failure, the mandated P6g red. Say so.
- `cd lint && go test -count=1 ./...` -- **`-count=1` is required**; a cached `ok` here is no
  measurement at all, and this module reads `font-catalogue.test.ts` as source text.
- `cd folio-designer && npx playwright test e2e/font-embed-boundary.spec.ts` -- expected: the
  one-asset-per-family assertion has been repaired and passes against the new population.
- For every one of the 78 new faces: re-hash its archive against the SHA-256 its family's NOTICE
  already pins, and re-hash the extracted file against what the new NOTICE records -- expected: an
  explicit "78 of 78" witness. A face whose bytes do not match the SHA-256 its NOTICE pins is worse
  than no face: stop rather than commit it.

**Manual checks (if no CLI):**
- Every new `public/fonts/<dir>/NOTICE.md` records the archive sha256, the source-file sha256, the
  shipped-file sha256 and the byte size, and `build-wasm.mjs:420-436`'s four rows parse from it.
- The generated `src/generated/runtime-fonts.css` carries one rule per declared face with no
  `font-weight` and no `font-style`, and `canvasFaceAssets` has one entry per rule.
