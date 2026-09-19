---
title: 'Example bundling pipeline with the Invoice example'
type: 'feature'
created: '2026-09-15'
status: 'done'
route: 'dispatch'
baseline_commit: '756686733132f3e38201fafb85b7ead355299e56'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/specs/spec-startup-templates/SPEC.md'
  - '{project-root}/_bmad-output/specs/spec-startup-templates/example-templates.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The startup dialog (SPEC CAP-1..CAP-4) needs example templates that ship offline with their sample data and an engine-true thumbnail, but the build bundles only `starter.folio` and has no way to render a template or rasterize a PDF.

**Approach:** Add a build stage that, for each listed example, renders `<id>.folio` against `<id>.sample.json` with the engine CLI in strict mode, fails the build on any diagnostic, rasterizes page 1 to PNG, and fingerprints all three files into the offline release behind a generated `example-assets.ts`. Prove it with the Invoice example.

## Boundaries & Constraints

**Always:**
- Example sources live in `folio-designer/public/templates/examples/`; the list of example ids is one array in the build stage (story 2 appends to it).
- Sample JSON stays a separate file; the `.folio` never embeds data or a path.
- Thumbnail comes only from the engine's PDF of that exact template+sample; never hand-drawn or committed.
- Any render diagnostic (warning or error), missing file, or non-zero CLI exit fails `build:wasm` with a message naming the example id and the CLI's stderr.
- Invoice uses only shipped faces (copy the starter's font chain), declares `version` `4.0` (qrcode), is in canonical form, and meets its row in `example-templates.md`: issuer/bill-to blocks, `items[]` table with qty, unit price, amount (amount precomputed in data) and a `footer: "sum"` amount column, subtotal/tax/total via `formatNumber`, issue/due dates via `formatDate` (RFC 3339 input), payment-reference `qrcode`. 6–10 items, fictional English data, neutral two-decimal currency.
- Raise `maximumCacheAssets` with stated headroom, not tuned to the count: 65 → 90 and `warnCacheAssets` 56 → 82, comment naming this story and the 12 slots four examples take.
- New devDependency `@napi-rs/canvas` pinned exactly to `1.0.8` (MIT, already in the lockfile via pdfjs-dist), with a licence/placement test mirroring the `fake-indexeddb` admission tests.

**Never:**
- No UI, dialog, or change to startup behaviour (stories 3–4).
- No other examples (story 2).
- No new entry in `offline-assets.ts` (the engine worker imports it) and no S1 load-screen row.
- No new `dependencies` entry; no rasterizer in `folio-go` non-test code (`nocompressor` lint).
- Do not place examples in `docs/examples/` (that triggers the guide-embedding test).

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Happy path | `invoice.folio` + `invoice.sample.json` valid | `runtime/invoice.<hash>.folio`, `.sample.<hash>.json`, `.thumbnail.<hash>.png` emitted; `example-assets.ts` exports them | N/A |
| Render warning | sample causes e.g. an overflow diagnostic | `build:wasm` exits non-zero | Error names `invoice` and prints the diagnostic line |
| Missing sample | `invoice.sample.json` absent | `build:wasm` exits non-zero | Error names the missing path |
| Invalid template | `.folio` fails to load | `build:wasm` exits non-zero | Error names `invoice` and the CLI stderr |

</frozen-after-approval>

## Code Map

- `folio-designer/scripts/build-wasm.mjs` -- emits `runtime/`; `fingerprint()` (~:77) content-addresses files; starter handling (~:74) and `documentation-assets.ts` emission (~:500) are the patterns to follow; Go is already required (`execFileSync('go', …)`).
- `folio-go/cmd/folio8/main.go` -- `folio8 render -data <json> -o <pdf> -strict <tpl>`; `-strict` exits 1 on any warning; diagnostics on stderr as `SEV CODE element=ID: msg`.
- `node_modules/pdfjs-dist/legacy/build/pdf.mjs` -- Node-side PDF rendering; uses `@napi-rs/canvas` for its canvas factory.
- `folio-designer/src/release-payload.ts:41-57` -- `minimumCacheAssets`/`maximumCacheAssets`/`warnCacheAssets`; each must stay `const <name> = <digits>` on its own line (read by `scripts/offline-release-contract.mjs`).
- `folio-designer/src/build-wasm.test.ts:~290` -- pins the seven generated emissions; add `example-assets.ts`.
- `folio-designer/.gitignore:146-154` -- `src/generated/*` entries; add `example-assets.ts`.
- `folio-designer/src/font-store.test.ts:~412-470` -- `fake-indexeddb` admission tests to mirror for `@napi-rs/canvas` (devDependencies only, not imported under `src/` shipping modules, LICENSE text checked).
- `folio-designer/public/templates/starter.folio` -- font chain to reuse.
- `fixtures/statement-5/input.folio`, `fixtures/qrcode-payments/input.folio`, `docs/examples/ruled-table.folio` -- shapes for table with footer sum, qrcode, text.
- `folio-go/docs_examples_test.go:47,83` -- `docsExamplePages` / `requireNoDiagnostics` helpers to reuse.
- `folio-go/serialize_template.go:15` -- `SerializeTemplate(ParseTemplate(bytes))` for the canonical-form assertion.

## Tasks & Acceptance

**Execution:**
- [x] `folio-designer/public/templates/examples/invoice.folio`, `invoice.sample.json` -- author the Invoice per Boundaries -- the proving example.
- [x] `folio-go/designer_examples_test.go` -- for each file in `../folio-designer/public/templates/examples/*.folio`: bytes equal `SerializeTemplate(ParseTemplate)` output, and render with its `.sample.json` has zero diagnostics -- canonical form and render health checked in `go test`, not only in the build.
- [x] `folio-designer/scripts/build-examples.mjs` (new, called from `build-wasm.mjs`) -- build the CLI once to a temp dir, render each example strict, rasterize page 1 at 264 px wide, fingerprint template/sample/thumbnail into `runtime/`, emit `src/generated/example-assets.ts` exporting `exampleAssets` as `readonly { id, template, sample, thumbnail }[]` of `?url` imports -- the pipeline.
- [x] `folio-designer/scripts/build-examples.test.mjs` -- failure paths from the I/O matrix (warning, missing sample, invalid template) each reject naming the example -- guards the build gate.
- [x] `folio-designer/src/release-payload.ts` -- raise bounds as stated, update comments -- room for four examples.
- [x] `folio-designer/package.json`, `package-lock.json` -- add `@napi-rs/canvas` devDependency `1.0.8` -- rasterizer.
- [x] `folio-designer/src/font-store.test.ts` (or a sibling test) -- admission tests for `@napi-rs/canvas` -- AD-26 policy.
- [x] `folio-designer/src/build-wasm.test.ts`, `.gitignore` -- register `example-assets.ts` -- generated-tree contract.

**Acceptance Criteria:**
- Given a clean checkout, when `npm run build` runs, then it succeeds, `dist/offline-release-manifest.json` lists the three Invoice assets, and `verify:offline` passes.
- Given the built thumbnail, when it is opened, then it is a PNG 264 px wide showing the Invoice's first page as rendered by the engine.
- Given `exampleAssets` is imported in a test, when read, then it has exactly one entry with id `invoice` and three URLs.
- Given `go test ./...` in `folio-go`, when run, then the designer-examples test passes for Invoice.

## Implementation Notes

- The `example-assets.ts` ignore line lives in the repository-root `.gitignore` (where every `src/generated/*` entry is), not `folio-designer/.gitignore` as the Code Map said.
- `src/main.tsx` gained a side-effect-only `import './generated/example-assets.ts'`: without an application importer Vite does not emit the three files and the release manifest omits them. No behaviour; story 3 replaces it with the dialog's real import.
- Asset-bound literals pinned elsewhere were updated with the bounds: `src/font-store.test.ts` (65 → 90), the over-bound payload in `src/release-payload.test.ts` (66 → 91), `src/vendor/pdfjs/PROVENANCE.md`.
- Invoice totals block moved down 24 pt after the first render overlapped the table's sum row; positions are fixed, so far longer sample data would overlap (the engine never pushes elements).
- Verified: `go test` designer-examples test passes; the only `go test ./...` failure, `internal/text` `TestCorpusMeetsP6ExerciseFloors` (P6g 7 < 20), fails identically at `baseline_commit` — pre-existing. Five story vitest files 51/51; `typecheck` 0; release manifest 68/90 with the three Invoice assets; thumbnail inspected (264 px, Invoice page 1).
- `npm run lint` exits 0 (8 pre-existing `only-export-components` warnings, none in touched files); one earlier exit 2 did not reproduce on two re-runs.
- Review patches (triage rows 1–3, 5, 9, 18, 20): `buildExamples` rasterizes every example before fingerprinting any; `verify-offline-release.mjs` gained `templateAssetFinding` — starter required by name (replacing the generic `.folio` class) and exactly one immutable template/sample/thumbnail per `exampleIds` entry, with fixture cases in its test; `build-examples.test.mjs` now asserts the positive control's PNG width, captures the warning error once, checks `exampleIds` equals the directory's `.folio` stems (each with a sample), and requires >2% non-white thumbnail pixels.
- Post-patch verification: `npm run build` 0 (incl. `verify:offline`); `npx vitest run` 1804/1804 in 85 files; `typecheck` 0; `lint` 0; `go test ./...` only the pre-existing `TestCorpusMeetsP6ExerciseFloors`.
- Risk: thumbnail bytes may differ across machines (pdf.js + Skia glyph rasterization), changing the release id; content-addressed, so nothing breaks.

## Spec Change Log

## Review Triage Log

| # | Layer | Finding | Verdict | Evidence | Route |
|---|---|---|---|---|---|
| 1 | blind, edge | Rasterize failure of a later example leaves earlier examples fingerprinted, contradicting "fails before anything is written" | low | Fingerprint runs inside the rasterize loop (`build-examples.mjs` stage loop); build-wasm clears `runtime/` next build, but the comment is false | patch |
| 2 | blind | Positive-control test named "thumbnail exactly the declared width" never checks width | low | Stub `fingerprint` returns the label; PNG deleted with the work dir; no IHDR read in that test | patch |
| 3 | blind | Warning test runs the stage twice on one `generatedDir` | low | Two `buildExamples` calls; assertions describe whichever ran last | patch |
| 4 | blind | No tests for id validation, no-PDF exit, `go build` failure | low | True but unlikely in everyday use (curated one-array ids); fix adds tests beyond a direct correction | rejected |
| 5 | blind, edge | `exampleIds` and the examples directory can drift | medium | Go test globs `*.folio`; build ships only `exampleIds`; story 2 adds files, and a missing id passes `go test` yet ships nothing | patch |
| 6 | blind, edge | Fixed totals/QR positions overlap a longer table; strict render does not catch overlap | low | Bundled 8-row sample renders clean (thumbnail inspected); overlap needs data the example does not ship; fix is a layout redesign | rejected |
| 7 | blind | `warnCacheAssets` comment says "eight below", old margin was 9 | false | The original rationale defines the margin as eight below the ceiling (56 vs 64); 90 − 82 = 8 matches it | rejected |
| 8 | blind | `PROVENANCE.md` "fails that budget outright" no longer re-derived | false | 68 assets + 32 viewer images = 100 > 90, so the sentence still holds; the parenthetical keeps the measured-time cap | rejected |
| 9 | blind, verification-gap | No automated check that the release carries the example assets; the `main.tsx` side-effect import is unguarded | medium | `verify-offline-release.mjs` checks documentation pages by stem but nothing for examples; manifest contents verified only by hand | patch |
| 10 | blind, edge | Runtime-emission test depends on build state / stale thumbnails | false | `build-wasm.mjs` removes `runtime/` before emitting; `npm test` runs `build:wasm` first; existing tests import generated modules the same way | rejected |
| 11 | blind | Thumbnail not byte-reproducible, churning release id | false | Two fresh rasterizations and the built thumbnail are byte-identical (`88abffc3…`); cross-machine variance is the recorded risk | rejected |
| 12 | blind | Subtotal shown twice (footer sum + Subtotal line) | low | Both are required by `example-templates.md`; the fix edits this build's spec | rejected |
| 13 | blind | `SOURCE_DATE_EPOCH` comment overclaims CLI = designer render | low | CLI uses `fonts.Shipped()`, the same faces the designer's engine embeds; cosmetic wording | rejected |
| 14 | edge | Rasterize error does not name the example id | low | True; every example shares one rasterizer, unlikely to mislead; fix adds a guard | rejected |
| 15 | edge | CLI hang has no timeout | low | No reachable hang shown; fix adds a guard | rejected |
| 16 | edge | Ambient `GOOS`/`GOARCH` cross-compiles the CLI | low | Unlikely in developer and CI environments; fix adds env handling | rejected |
| 17 | edge | Example id colliding with a runtime slot stem | low | Ids are curated in one array; fix adds a reserved-name guard | rejected |
| 18 | edge | Verifier's required `.folio` class is now satisfied by an example, masking a missing starter | medium | `verify-offline-release.mjs:126` accepts any asset ending `.folio` | patch |
| 19 | edge | Height > width assertion fails for a future landscape example | false | Every current and planned example is portrait; no landscape case is reachable | rejected |
| 20 | verification-gap | A blank white thumbnail passes every thumbnail assertion | medium | Pre-verified: tests check signature, width and aspect only, never pixel content | patch |

## Design Notes

Why a separate generated module: `offline-assets.ts` is imported by the engine worker, so adding examples there would pull them into the worker graph — `documentation-assets.ts` was split out for the same reason.

Why raise the asset bound: epics.md records `maximumCacheAssets` as a defensive shape bound, legitimately raised "with stated headroom rather than tuning it to fit the new count". The last build already sits at 65/65.

## Verification

**Commands:**
- `cd folio-go && go test ./...` -- expected: pass, including the designer-examples test.
- `cd folio-designer && npm run build:wasm` -- expected: exits 0; `src/generated/example-assets.ts` and three `invoice.*` files under `src/generated/runtime/`.
- `cd folio-designer && npx vitest run` -- expected: pass.
- `cd folio-designer && npm run build` -- expected: exits 0 including `verify:offline`.
- `cd folio-designer && npm run lint && npm run typecheck` -- expected: pass.

**Manual checks (if no CLI):**
- Open the emitted Invoice thumbnail PNG and confirm it matches the rendered Invoice page 1.
