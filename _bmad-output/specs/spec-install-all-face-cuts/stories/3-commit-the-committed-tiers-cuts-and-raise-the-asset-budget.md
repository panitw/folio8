---
title: "Instance the committed tier's cuts and raise the asset budget"
type: 'feature'
created: '2026-09-19'
status: 'draft'
route: 'dispatch'
review_loop_iteration: 0
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
publish — **112 declared cuts, 81 of them new** (25 families × 4, 6 families × 2; measured against
the `font-index.json` style snapshot, see Design Notes for its limits). Each cut becomes a
`font-catalogue.json` row carrying its own `style`, its own directory, its own LICENSE and
NOTICE.md, and its own deferred asset — the same shape a Regular row already has. The offline
release's `maximumCacheAssets` rises from 90 to fit the measured result, derived and not retyped,
with `warnCacheAssets` moved with it. The blocking core tier does not move.

**Planned against uncommitted work.** This spec was planned against the working tree as it stands,
not against HEAD `431e288`: ~25 files of spec-deferred-offline-cache story 5 are in flight,
including `folio-go/fonts/fonts.go` and new files under `folio-go/fonts/`. It also **assumes
stories 1 and 2 of this spec have landed** — story 1's family-face-set model and story 2's per-cut
chain entry are what make a multi-cut catalogue consumable.

## Boundaries & Constraints

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
- **Every family-keyed collapse must be re-keyed or proven safe.** `font-index.ts:119`
  `localByFamily = new Map(catalogueFaces.map(...))` is **last-wins with no tie-break** — silent
  today only because the catalogue is one Regular per family. It is this story's to fix, not story
  1's; story 1's dispatch names `regularFilename` and `mostRecentlyFetched`, not this.
- **The core BYTE ceiling is live and thin.** `maximumCoreCacheBytes = 6553600`
  (`release-payload.ts:179`) has **~155,543 Brotli bytes of headroom** at the last build.
  `src/generated/font-catalogue.ts` inlines one ~4 KB licence text **per face**
  (`build-wasm.mjs:374`) plus a per-face `copyright` and `source` string, and that module is
  bundled into the **core** tier. 81 more faces is ~324 KB of raw bundle text. The licence texts
  are near-duplicates over three SPDX ids so Brotli should crush them, but the `source` strings are
  unique. **Measure it; do not assume it.**
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
  release assets** (`/assets/roboto-bold.*`, `roboto-italic.*`, `roboto-bold-italic.*`). See Q3.
- Do not add any face to the core tier, to `CORE_CATALOGUE_FACE_IDS`, or to `public/templates/starter.folio`.
- Do not add a `font-weight` or `font-style` descriptor to any emitted `@font-face` rule.
- No new weight beyond the four cuts, no variable fonts, no synthetic bold or oblique, no CJK
  catalogue change (Noto Sans SC stays on the shipped-face path).
- Do not change the `.folio` format, the chain mechanism, or the `fonts` map syntax.
- Do not touch the document, any engine command, or the Add font dialog's pre-pick display (story 5).

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Four-cut family | Inter: upstream publishes 400/700/400i/700i | Four catalogue rows (`inter`, `interbold`, `interitalic`, `interbolditalic`), all `family: "Inter"` with distinct `style`; four deferred assets; four `@font-face` rules under the derived names `Inter` / `Inter Bold` / `Inter Italic` / `Inter Bold Italic` | N/A |
| Family offered once | Four Inter rows in `catalogueFaces` | The Add font dialog lists `Inter` exactly once (`font-index.test.ts:189-197`), `addableFamilyCount` counts it once, and `localByFamily` resolves each cut by `style` rather than last-wins | N/A |
| Core bundle grows | 81 more inlined licence/copyright/source strings in `font-catalogue.ts` | Core Brotli stays at or under `maximumCoreCacheBytes` 6,553,600 | `generate-offline-release.mjs:222-224` throws naming the weight and the ceiling |
| Italic-less family | Oswald: upstream publishes 400/700, no 400i | Two rows only; `Oswald Italic` is never declared, never fetched, never offered | N/A |
| Row style disagrees with bytes | A row declares `Bold`; the binary reports subfamily `Regular`, `usWeightClass` 400 | `font-catalogue.test.ts` reds naming the row, the declared style and the subfamily read from the bytes | Test failure |
| Two rows, one (family, style) | Two rows both declare `Inter` / `Bold` | The re-keyed `build-wasm.mjs:313` throws before any asset is emitted | Build failure, naming family and style |
| Cuts added, budget not moved | Release emits >90 assets | `verify-offline-release.mjs:118` fails against the derived `maximumCacheAssets` | Build failure, count and bound named |
| Budget line reformatted | `const maximumCacheAssets = 168` split or commented | `readDeclaredConstant` finds ≠1 live match and throws | Build failure, naming the reader |
| Core tier unchanged | 78–81 new deferred assets emitted | `coreAssetCount` stays 30; `maximumCoreCacheBytes` (6,553,600) untouched | N/A |
| Catalogue row count drifts from emitted assets | `font-catalogue.json` and the release disagree | `generate-offline-release.mjs:207` throws naming both counts | Build failure |

</frozen-after-approval>

## Open Questions

- **Q1 — How are the new cuts acquired? SPEC.md's constraint describes a mechanism the catalogue
  tier has never used.** SPEC.md says *"A committed family's cuts are instanced at build time, never
  fetched… Bold is the same variable source pinned at `wght=700`, italic comes from the family's
  italic VF source"*, and story 3's dispatch note says `tools/fontgen/instance_faces.py` *"is that
  pipeline"*. **Measured, both are false for the catalogue.** `instance_faces.py` has seven
  `UPSTREAM` entries, all Noto, and writes to `folio-go/fonts/<dir>/` — it never mentions
  `public/fonts` and has no `--out` default that reaches it. All 31 catalogue Regulars are **direct
  upstream static downloads** whose NOTICE.md records *"Relation to source: copied unmodified, no
  derivation"*; the `arimo` NOTICE says outright *"This repository cannot derive a face at all"*.
  Of the 44 directories under `public/fonts/`, only the 7 shipped Noto faces are instanced, and
  those were hand-copied from `folio-go/fonts/` with no script performing or verifying the copy.
  Options:
  - **(A) Extract the upstream statics from the archives already pinned in each NOTICE.** Every
    catalogue NOTICE already records the release URL, the archive sha256 and the path inside it;
    the Bold/Italic/BoldItalic statics sit beside the Regular in the same archive. Consequence:
    matches the tier's actual acquisition model, no new build tooling, no fontTools dependency, but
    contradicts a written Constraint in the canonical SPEC, and ~31 archives must be re-fetched.
  - **(B) Build a catalogue-side instancing pipeline as SPEC.md constrains.** Consequence: honours
    the contract as written, gives byte-deterministic replay under `SOURCE_DATE_EPOCH` with
    src/out sha256 asserted both ways, but means acquiring ~28 variable sources plus their italic
    VFs, extending `instance_faces.py` (or a sibling) to a second output tree, and shipping faces
    whose bytes match no upstream release — for families that already publish the statics.
  - **(C) Mixed:** statics where upstream publishes them, instancing only where it does not.
    Consequence: two provenance shapes in one tier; NOTICE.md already has three non-uniform shapes
    and this adds a fourth split.
  - This contradicts the canonical contract, so it is a human's call, not the build's.

- **Q2 — Confirm the measured budget and the headroom.** Measured: the 31 families publish **112
  cuts**, so the catalogue grows by **81 faces** (78 if Q3 resolves to reuse). Today's release is
  **80 assets** (30 core / 50 deferred); it becomes **161** (or **158**). `maximumCacheAssets` must
  therefore roughly double, from 90. Options:
  - **(A) `maximumCacheAssets = 168`, `warnCacheAssets = 160`** — 158 measured plus the same
    10-slot reserve the current 90 carries over today's 80, with the warning eight below the
    ceiling, which is the existing "one comparable batch" margin. Consequence: consistent with both
    prior raises; assumes Q3 resolves to reuse.
  - **(B) `maximumCacheAssets = 171`, `warnCacheAssets = 163`** — the same arithmetic over 161.
    Consequence: correct if Q3 resolves to duplicate.
  - **(C) A number fitted to the count with no reserve.** Consequence: the next unrelated batch
    reds the build.
  - Either way the number must be re-derived from the emitted manifest at implementation time,
    because the per-family cut counts carry a provenance caveat (Design Notes).

- **Q3 — Roboto's three cuts already ship. Does the catalogue declare them anyway?** `Roboto Bold`,
  `Roboto Italic` and `Roboto Bold Italic` exist as committed directories under `public/fonts/` and
  as **core** release assets. Options:
  - **(A) The catalogue does not declare them; the existing core assets serve them.** Consequence:
    3 fewer slots and ~1.1 MiB less repo growth, but Roboto becomes the one family whose cuts come
    from a different place than every other family's, and `held-local-faces.ts` / the font browser
    must join two sources for one family.
  - **(B) The catalogue declares them, pointing at the existing directories.** Consequence: uniform
    model, but `fingerprint()` emits a second dist asset with byte-identical content (+3 slots,
    ~1.1 MiB of duplicate release payload) — the "second copy" the dispatch note warns against, at
    the asset layer rather than the repo layer.
  - **(C) The catalogue declares them and the shipped core copies are retired.** Consequence:
    cleanest model, but moves Roboto's cuts out of the blocking tier — the starter template's body
    text — and therefore moves the 30/30 core pin, which SPEC.md forbids.

- **Q4 — Is this one story?** The footprint measured is **81 new committed font binaries, 81 new
  NOTICE.md + LICENSE pairs, ~21.8 MiB of repo growth**, four interlocked population floors, a
  family-uniqueness guard, the CSS and canvas-map emitters, the licence census (~81 new rows), an
  e2e one-asset-per-family premise, and the budget. The code change is modest; the **acquisition
  and provenance work is not**. Options:
  - **(A) One story, whole batch.** Consequence: CAP-3 lands complete in one commit; a very large,
    largely mechanical diff.
  - **(B) Mechanism first, binaries after.** Land the `style` field, the schema, the retired
    assertion, the emitters and the budget against a small proving tranche (say Roboto plus two
    families), then acquire the rest in a follow-on. Consequence: every guard is exercised on a
    reviewable diff; CAP-3 is only partly true until the follow-on lands, so story 5's pre-pick
    display would advertise cuts that are not there yet.
  - **(C) Split by tranche** (Google-hosted families, then the rest). Consequence: three medium
    commits; the budget constant moves three times.

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
- `tools/fontgen/instance_faces.py` unless Q1 resolves to (B) or (C).
- `folio-designer/public/templates/starter.folio`, the core tier, `CORE_CATALOGUE_FACE_IDS`.

## Tasks & Acceptance

**Execution:**
- [ ] `folio-designer/public/fonts/` -- acquire and commit each new cut per Q1's ruling: one directory per cut, exactly three files (the `.ttf`, one `LICENSE*`, one `NOTICE.md`) -- the tier's existing per-face provenance shape; `build-wasm.mjs:383-388` requires exactly one `LICENSE*` per directory.
- [ ] `folio-designer/font-catalogue.json` -- add a `style` key to every row and one row per new cut, `family` being the cut's own CSS family name (`Inter Bold`) -- `style` is what the retired assertion is replaced by; per-cut family names keep `build-wasm.mjs:313` and the family-keyed canvas map intact.
- [ ] `folio-designer/scripts/build-wasm.mjs` -- add `style` to the `:296` required-field list with a closed-set check (`Regular|Bold|Italic|BoldItalic`); re-key the `:313` uniqueness guard (and its `:294` seed) to **(family, style)**; derive the `@font-face` family name per cut at `:584` without adding a descriptor; replace the `:460` `style: "Regular"` stamp with the row's value; rewrite the `:107-126` rationale and the stale 31-face/slot-count prose at `:335`, `:341`, `:374`, `:449-450`, `:595` -- the comment at `:110-112` asserts a rule this story ends.
- [ ] `folio-designer/src/font-catalogue.test.ts` -- replace `:461-481` with a per-row claim that the binary's subfamily, `usWeightClass`, `fsSelection` bold/italic bits, `macStyle` and `italicAngle` match the declared `style`; keep `:476`/`:478-479`; update the `:35-40` prose; raise the `:335` floor -- an invariant is being retired, not deleted; `:654-760` is the working pattern.
- [ ] `folio-designer/src/font-index.test.ts`, `font-name-table.test.ts`, `font-provenance.test.ts` -- raise the other three population floors in the same commit -- their own comments say all four move together.
- [ ] `folio-designer/src/font-index.ts` -- re-key `localByFamily` (`:119`) so a family resolves to its face SET and each cut is found by `style`, and make `addableFamilyCount` (`:172`) count distinct families rather than catalogue rows -- `:119` is a silent last-wins `Map` today and `:172` would report four Inters; `:189-197` must stay true.
- [ ] `folio-designer/src/release-payload.ts` -- raise `maximumCacheAssets` (`:72`) and `warnCacheAssets` (`:91`) to Q2's ruling, each still `^const <name> = <digits>$` on its own line, with a rationale comment beside it naming the measured count -- the reader at `offline-release-contract.mjs:204-208` is line-anchored and throws on ≠1 live match.
- [ ] `folio-designer/scripts/generate-offline-release.mjs` -- `:237` `brotli.catalogue.familyCount` now counts faces, not families; rename it and move `verify-offline-release.mjs:409` with it -- a field that lies is worse than one that reds.
- [ ] `folio-designer/e2e/font-embed-boundary.spec.ts`, `src/held-local-faces.ts`, `scripts/offline-release-contract.test.mjs`, `src/App.font-store.test.tsx`, `src/font-binary-identity.test.ts` -- repair the family↔asset 1:1 premise in each -- measured: each assumes one catalogue face per family.
- [ ] `folio-go/fonts/fonts_test.go:89-92`, `folio-go/fonts/fonts.go:72-80`, `folio-go/component_commands.go:4413` -- rewrite the "catalogue stays Regular-only" rationale -- it is now false; leave the tests' behaviour alone.
- [ ] Re-check `licencesignature_test.go:735,:856` still find their three source-text needles, and refresh the line-number citations at `licencesignature.go:16-18`, `component_commands.go:4341-4342`, `fonts/accounting_test.go:103` -- they read `font-catalogue.test.ts` by line.

**Acceptance Criteria:**
- Given the 31 committed families, when the catalogue is rebuilt, then every cut that family publishes is declared with its own `style`, and a family publishing no italic declares none.
- Given a rebuilt release, when `npm run build` runs, then `verify:offline` passes, the core tier is still exactly 30 assets, `maximumCoreCacheBytes` is not exceeded, and the emitted asset count is at or below the newly derived `maximumCacheAssets`.
- Given any catalogue row, when `font-catalogue.test.ts` runs, then the binary's own subfamily and weight class are asserted against that row's declared `style`, and a row mislabelled `Bold` over a Regular binary reds naming the row.
- Given the release manifest, when the tiers are classified, then every new cut is `deferred`, `CORE_CATALOGUE_FACE_IDS` still resolves to `catalogue-roboto` alone, and the core tier's Brotli weight is still under `maximumCoreCacheBytes` despite the larger inlined licence/provenance text in `font-catalogue.ts`.
- Given four cuts of one family in the catalogue, when the Add font dialog lists families, then that family appears exactly once and each cut resolves to the row whose `style` matches — never to whichever row sorted last.
- Given an author offline on a committed family with a bold, when they press **B** after stories 1 and 2, then the bold paints with no diagnostic.

## Implementation Notes

## Spec Change Log

## Review Triage Log

## Design Notes

**Where the 112 came from, and what it does not prove.** Per-family cut availability was read from
`folio-designer/font-index.json` (`snapshotDate` 2026-09-03), taking weight `700` as bold, `400i`
as italic and `700i` as bold italic. 25 families yield four cuts, 6 yield two (no italic upstream):
Fira Code, Noto Sans Thai Looped, Noto Serif Thai, Oswald, Roboto Slab, Space Grotesk.

Two caveats, both material to Q2's number:
1. **Inter Display and Source Serif 4 Display are absent from the index** and were *assumed* to
   publish four cuts from their upstream archives (`rsms/inter v4.1 extras/ttf/`,
   `adobe-fonts/source-serif 4.005R TTF/`). Not verified — no network was used.
2. **The index is a Google Fonts snapshot; the catalogue is acquired from upstream project
   archives.** For Cascadia, Geist, Intel One Mono, Fira Code, JetBrains Mono, Inter, Source* and
   Ubuntu the two can disagree. Each family's real cut set must be confirmed against the archive
   its own NOTICE.md pins.

**Measured figures (all from the working tree, not HEAD).**

| Quantity | Today | After |
|---|---|---|
| Catalogue rows | 31 | 112 |
| New committed faces | — | 81 (78 if Q3 = reuse) |
| Release assets | 80 (30 core / 50 deferred) | 161 (158 if Q3 = reuse) |
| Catalogue raw bytes | 8,466,552 (8.07 MiB) | ~31.3 MiB (+22.85 MiB, Q3 = reuse) |
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
licence text per face into the **core** bundle, against ~155 KB of Brotli headroom. If 81 more
faces breach `maximumCoreCacheBytes`, the fix is to deduplicate the licence texts by SPDX id —
`build-wasm.mjs:341-342` already observes that 31 faces emit 31 texts over only three identifiers —
rather than to move the ceiling, which guards the first-load screen.

## Verification

**Commands:**
- `cd folio-designer && npm run build` -- expected: `build:wasm`, `tsc -b`, `vite build`,
  `build:offline` and `verify:offline` all succeed; the emitted manifest's `assets.length` is at or
  below the new `maximumCacheAssets`, `brotli.core.assetCount` is 29 and `coreAssetCount` is 30.
  **Print the emitted asset count AND `brotli.core.totalBytes` and quote both in the report** —
  the first is what Q2's number must be re-derived from; the second is the ~155 KB-headroom core
  byte ceiling the enlarged `font-catalogue.ts` bundle lands on.
- `cd folio-designer && npm run typecheck && npm run lint && npm test` -- expected: green; 4
  pre-existing `only-export-components` lint warnings are not a regression. Report pass counts.
- `cd folio-go && go test ./...` -- expected: exactly one failure, the mandated P6g red. Say so.
- `cd lint && go test -count=1 ./...` -- **`-count=1` is required**; a cached `ok` here is no
  measurement at all, and this module reads `font-catalogue.test.ts` as source text.
- `cd folio-designer && npx playwright test e2e/font-embed-boundary.spec.ts` -- expected: the
  one-asset-per-family assertion has been repaired and passes against the new population.
- If Q1 resolves to (B) or (C): `python3 tools/fontgen/instance_faces.py --sources DIR --verify-only`
  -- expected: an explicit "N of N" coverage witness. Note that
  `TestShippedFacesReproduceFromUpstream` fails under `-tags=matrix` without local `fontTools`; that
  is environmental, not a regression.

**Manual checks (if no CLI):**
- Every new `public/fonts/<dir>/NOTICE.md` records the archive sha256, the source-file sha256, the
  shipped-file sha256 and the byte size, and `build-wasm.mjs:420-436`'s four rows parse from it.
- The generated `src/generated/runtime-fonts.css` carries one rule per declared face with no
  `font-weight` and no `font-style`, and `canvasFaceAssets` has one entry per rule.
